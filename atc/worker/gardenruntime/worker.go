package gardenruntime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker/gardenruntime/gclient"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/worker/baggageclaim"
	"golang.org/x/sync/errgroup"
)

type Streamer interface {
	Stream(ctx context.Context, src runtime.Artifact, dst runtime.Volume) error
	StreamFile(ctx context.Context, src runtime.Artifact, path string) (io.ReadCloser, error)
}

type Worker struct {
	streamer Streamer

	dbWorker     db.Worker
	gardenClient gclient.Client
	bcClient     baggageclaim.Client

	db DB
}

type DB struct {
	VolumeRepo                    db.VolumeRepository
	TaskCacheFactory              db.TaskCacheFactory
	WorkerTaskCacheFactory        db.WorkerTaskCacheFactory
	ResourceCacheFactory          db.ResourceCacheFactory
	WorkerBaseResourceTypeFactory db.WorkerBaseResourceTypeFactory
	LockFactory                   lock.LockFactory
}

func NewWorker(dbWorker db.Worker, gardenClient gclient.Client, bcClient baggageclaim.Client, db DB, streamer Streamer) *Worker {
	return &Worker{
		streamer: streamer,

		dbWorker:     dbWorker,
		gardenClient: gardenClient,
		bcClient:     bcClient,

		db: db,
	}
}

func (worker *Worker) Name() string {
	return worker.dbWorker.Name()
}

func (worker *Worker) FindOrCreateContainer(
	ctx context.Context,
	owner db.ContainerOwner,
	metadata db.ContainerMetadata,
	containerSpec runtime.ContainerSpec,
	stderr io.Writer,
) (runtime.Container, []runtime.VolumeMount, error) {
	c, mounts, err := worker.findOrCreateContainer(ctx, owner, metadata, containerSpec, stderr)
	if err != nil {
		return nil, nil, fmt.Errorf("find or create container on worker %s: %w", worker.Name(), err)
	}
	return c, mounts, err
}

func (worker *Worker) findOrCreateContainer(
	ctx context.Context,
	owner db.ContainerOwner,
	metadata db.ContainerMetadata,
	containerSpec runtime.ContainerSpec,
	stderr io.Writer,
) (runtime.Container, []runtime.VolumeMount, error) {
	logger := lagerctx.FromContext(ctx)
	fmt.Fprintf(stderr, "before FindContainer, owner=%v\n", owner)
	creatingContainer, createdContainer, err := worker.dbWorker.FindContainer(owner)
	if err != nil {
		return nil, nil, fmt.Errorf("find in db: %w", err)
	}

	// ensure either creatingContainer or createdContainer exists
	var containerHandle string
	if creatingContainer != nil {
		containerHandle = creatingContainer.Handle()
	} else if createdContainer != nil {
		containerHandle = createdContainer.Handle()
	} else {
		fmt.Fprintf(stderr, "before creating-container-in-db\n")
		logger.Debug("creating-container-in-db")
		creatingContainer, err = worker.dbWorker.CreateContainer(
			owner,
			metadata,
		)
		if err != nil {
			logger.Error("failed-to-create-container-in-db", err)
			return nil, nil, fmt.Errorf("create container: %w", err)
		}
		logger.Debug("created-creating-container-in-db")
		fmt.Fprintf(stderr, "created-creating-container-in-db\n")
		containerHandle = creatingContainer.Handle()
	}

	logger = logger.WithData(lager.Data{"container": containerHandle})

	gardenContainer, err := worker.gardenClient.Lookup(containerHandle)
	if err != nil {
		if !errors.As(err, &garden.ContainerNotFoundError{}) {
			logger.Error("failed-to-lookup-creating-container-in-garden", err)
			return nil, nil, err
		}
	}

	// if createdContainer exists, gardenContainer should also exist
	if createdContainer != nil {
		logger = logger.WithData(lager.Data{"container": containerHandle})
		logger.Debug("found-created-container-in-db")

		if gardenContainer == nil {
			return nil, nil, garden.ContainerNotFoundError{Handle: containerHandle}
		}
		return worker.constructContainer(
			ctx,
			createdContainer,
			gardenContainer,
		)
	}

	// we now have a creatingContainer. If a gardenContainer does not exist, we
	// will create one. If it does exist, we will transition the creatingContainer
	// to created and return a worker.Container
	if gardenContainer == nil {
		gardenContainer, err = worker.createGardenContainer(ctx, containerSpec, creatingContainer, stderr)
		if err != nil {
			logger.Error("failed-to-create-container-in-garden", err)
			markContainerAsFailed(logger, creatingContainer)
			return nil, nil, err
		}
	}

	logger.Debug("created-container-in-garden")

	createdContainer, err = creatingContainer.Created()
	if err != nil {
		logger.Error("failed-to-mark-container-as-created", err)
		_ = worker.gardenClient.Destroy(containerHandle)
		return nil, nil, err
	}

	logger.Debug("created-container-in-db")
	metric.Metrics.ContainersCreated.Inc()

	return worker.constructContainer(
		ctx,
		createdContainer,
		gardenContainer,
	)
}

func (worker *Worker) createGardenContainer(
	ctx context.Context,
	containerSpec runtime.ContainerSpec,
	creatingContainer db.CreatingContainer,
	stderr io.Writer,
) (gclient.Container, error) {
	logger := lagerctx.FromContext(ctx)
	fmt.Fprintf(stderr, "before fetchImageForContainer\n")
	fetchedImage, err := worker.fetchImageForContainer(
		ctx,
		containerSpec.ImageSpec,
		containerSpec.TeamID,
		creatingContainer,
		stderr,
	)
	if err != nil {
		logger.Error("failed-to-fetch-image-for-container", err)
		markContainerAsFailed(logger, creatingContainer)
		return nil, err
	}

	volumeMounts, err := worker.createVolumes(ctx, fetchedImage.Privileged, creatingContainer, containerSpec, stderr)
	if err != nil {
		logger.Error("failed-to-create-volume-mounts-for-container", err)
		markContainerAsFailed(logger, creatingContainer)
		return nil, err
	}

	bindMounts, err := worker.getBindMounts(ctx, volumeMounts, containerSpec)
	if err != nil {
		logger.Error("failed-to-create-bind-mounts-for-container", err)
		markContainerAsFailed(logger, creatingContainer)
		return nil, err
	}

	fmt.Fprintf(stderr, "creating-garden-container\n")
	logger.Debug("creating-garden-container")

	gardenContainer, err := worker.gardenClient.Create(
		garden.ContainerSpec{
			Handle:     creatingContainer.Handle(),
			RootFSPath: fetchedImage.URL,
			Privileged: fetchedImage.Privileged,
			BindMounts: bindMounts,
			Limits:     toGardenLimits(containerSpec.Limits),
			Env:        worker.containerEnv(containerSpec, fetchedImage),
			Properties: garden.Properties{
				userPropertyName: fetchedImage.Metadata.User,
			},
		})
	if err != nil {
		logger.Error("failed-to-create-container-in-garden", err)
		markContainerAsFailed(logger, creatingContainer)
		return nil, err
	}

	return gardenContainer, nil
}

func (worker *Worker) containerEnv(containerSpec runtime.ContainerSpec, fetchedImage FetchedImage) []string {
	env := append(fetchedImage.Metadata.Env, containerSpec.Env...)

	if worker.dbWorker.HTTPProxyURL() != "" {
		env = append(env, fmt.Sprintf("http_proxy=%s", worker.dbWorker.HTTPProxyURL()))
	}

	if worker.dbWorker.HTTPSProxyURL() != "" {
		env = append(env, fmt.Sprintf("https_proxy=%s", worker.dbWorker.HTTPSProxyURL()))
	}

	if worker.dbWorker.NoProxy() != "" {
		env = append(env, fmt.Sprintf("no_proxy=%s", worker.dbWorker.NoProxy()))
	}

	return env
}

func (worker *Worker) constructContainer(
	ctx context.Context,
	createdContainer db.CreatedContainer,
	gardenContainer gclient.Container,
) (Container, []runtime.VolumeMount, error) {
	logger := lagerctx.FromContext(ctx).WithData(
		lager.Data{
			"container": createdContainer.Handle(),
			"worker":    worker.Name(),
		},
	)

	createdVolumes, err := worker.db.VolumeRepo.FindVolumesForContainer(createdContainer)
	if err != nil {
		logger.Error("failed-to-find-container-volumes", err)
		return Container{}, nil, err
	}

	var volumeMounts []runtime.VolumeMount
	for _, dbVolume := range createdVolumes {
		if strings.HasPrefix(dbVolume.Path(), streamedVolumePathPrefix) {
			// streamed volumes aren't directly mounted to the container
			continue
		}
		volumeLogger := logger.Session("volume", lager.Data{
			"handle": dbVolume.Handle(),
		})

		volume, found, err := worker.LookupVolume(ctx, dbVolume.Handle())
		if err != nil {
			volumeLogger.Error("failed-to-lookup-volume", err)
			return Container{}, nil, err
		}

		if !found {
			err := MountedVolumeMissingFromWorker{
				Handle:     dbVolume.Handle(),
				WorkerName: worker.Name(),
			}
			volumeLogger.Error("volume-is-missing-on-worker", err, lager.Data{"handle": dbVolume.Handle()})
			return Container{}, nil, err
		}

		volumeMounts = append(volumeMounts, runtime.VolumeMount{
			Volume:    volume.(Volume),
			MountPath: dbVolume.Path(),
		})
	}
	return Container{GardenContainer: gardenContainer, DBContainer_: createdContainer}, volumeMounts, nil
}

// creates volumes required to run any step:
// * scratch (empty volume)
// * working dir (i.e. spec.Dir, empty volume)
// * inputs
//   * local volumes are COW'd
//   * remote volumes are streamed into an empty volume, then COW'd (only COW is mounted)
// * outputs (empty volumes)
// * caches (COW if exists, empty otherwise)
func (worker *Worker) createVolumes(
	ctx context.Context,
	privileged bool,
	creatingContainer db.CreatingContainer,
	spec runtime.ContainerSpec,
	stderr io.Writer,
) ([]runtime.VolumeMount, error) {
	var volumeMounts []runtime.VolumeMount

	scratchVolume, err := worker.findOrCreateVolumeForContainer(
		ctx,
		baggageclaim.VolumeSpec{
			Strategy:   baggageclaim.EmptyStrategy{},
			Privileged: privileged,
		},
		creatingContainer,
		spec.TeamID,
		"/scratch",
		stderr,
	)
	if err != nil {
		return nil, err
	}

	scratchMount := runtime.VolumeMount{
		Volume:    scratchVolume,
		MountPath: "/scratch",
	}

	volumeMounts = append(volumeMounts, scratchMount)

	inputVolumeMounts, inputDestinationPaths, err := worker.cloneInputVolumes(ctx, spec, privileged, creatingContainer)
	if err != nil {
		return nil, err
	}

	outputVolumeMounts, err := worker.createOutputVolumes(ctx, spec, privileged, creatingContainer, inputDestinationPaths)
	if err != nil {
		return nil, err
	}

	cacheVolumeMounts, err := worker.cloneCacheVolumes(ctx, spec, privileged, creatingContainer)
	if err != nil {
		return nil, err
	}

	ioVolumeMounts := append(inputVolumeMounts, outputVolumeMounts...)
	ioVolumeMounts = append(ioVolumeMounts, cacheVolumeMounts...)

	// if the working dir is already mounted, we can just re-use that volume.
	// otherwise, we must create a new empty volume
	if spec.Dir != "" && !anyMountTo(spec.Dir, ioVolumeMounts) {
		workdirVolume, err := worker.findOrCreateVolumeForContainer(
			ctx,
			baggageclaim.VolumeSpec{
				Strategy:   baggageclaim.EmptyStrategy{},
				Privileged: privileged,
			},
			creatingContainer,
			spec.TeamID,
			spec.Dir,
			io.Discard,
		)
		if err != nil {
			return nil, err
		}

		volumeMounts = append(volumeMounts, runtime.VolumeMount{
			Volume:    workdirVolume,
			MountPath: spec.Dir,
		})
	}

	sort.Sort(byMountPath(ioVolumeMounts))
	volumeMounts = append(volumeMounts, ioVolumeMounts...)

	return volumeMounts, nil
}

type mountableLocalInput struct {
	cowParent Volume
	mountPath string
}

type remoteInput struct {
	volume    runtime.Artifact
	mountPath string
}

func (worker *Worker) cloneInputVolumes(
	ctx context.Context,
	spec runtime.ContainerSpec,
	privileged bool,
	container db.CreatingContainer,
) ([]runtime.VolumeMount, map[string]bool, error) {
	inputDestinationPaths := make(map[string]bool)

	var localInputs []mountableLocalInput
	var remoteInputs []remoteInput

	for _, input := range spec.Inputs {
		volume, ok, err := worker.findVolumeForArtifact(ctx, spec.TeamID, input.Artifact, io.Discard)
		if err != nil {
			return nil, nil, err
		}

		cleanedInputPath := filepath.Clean(input.DestinationPath)
		inputDestinationPaths[cleanedInputPath] = true

		if ok && volume.DBVolume().WorkerName() == worker.Name() {
			localInputs = append(localInputs, mountableLocalInput{
				cowParent: volume.(Volume),
				mountPath: input.DestinationPath,
			})
		} else {
			remoteInputs = append(remoteInputs, remoteInput{
				volume:    input.Artifact,
				mountPath: input.DestinationPath,
			})
		}
	}

	locallyClonedVolumes, err := worker.streamRemoteInputVolumes(ctx, spec, privileged, container, remoteInputs)
	if err != nil {
		return nil, nil, err
	}
	// after we stream the remote volumes, they become "local" inputs. note
	// that we can't mount the streamed volumes directly, since those streamed
	// volumes may be cached locally - if we were to mount the raw volume and
	// if the container modifies the volume in any way, it'd affect subsequent
	// steps.
	localInputs = append(localInputs, locallyClonedVolumes...)

	mounts, err := worker.cloneLocalInputVolumes(ctx, spec, privileged, container, localInputs)
	if err != nil {
		return nil, nil, err
	}

	return mounts, inputDestinationPaths, nil
}

func (worker *Worker) cloneLocalInputVolumes(
	ctx context.Context,
	spec runtime.ContainerSpec,
	privileged bool,
	container db.CreatingContainer,
	inputs []mountableLocalInput,
) ([]runtime.VolumeMount, error) {
	mounts := make([]runtime.VolumeMount, len(inputs))

	for i, input := range inputs {
		cowVolume, err := worker.findOrCreateCOWVolumeForContainer(
			ctx,
			privileged,
			container,
			input.cowParent,
			spec.TeamID,
			input.mountPath,
			io.Discard,
		)
		if err != nil {
			return nil, err
		}
		mounts[i] = runtime.VolumeMount{
			Volume:    cowVolume,
			MountPath: input.mountPath,
		}
	}

	return mounts, nil
}

func (worker *Worker) streamRemoteInputVolumes(
	ctx context.Context,
	spec runtime.ContainerSpec,
	privileged bool,
	container db.CreatingContainer,
	inputs []remoteInput,
) ([]mountableLocalInput, error) {
	logger := lagerctx.FromContext(ctx)
	if len(inputs) == 0 {
		return nil, nil
	}
	mounts := make([]mountableLocalInput, len(inputs))

	ctx, span := tracing.StartSpan(ctx, "worker.streamRemoteInputVolumes", tracing.Attrs{"container_id": container.Handle()})
	defer span.End()

	g, groupCtx := errgroup.WithContext(ctx)

	for i, input := range inputs {
		// capture loop vars so each goroutine gets its own copy
		i, input := i, input

		// create an empty volume to stream-in the remote volume. this volume
		// will only be used as a parent volume (i.e. it won't be directly
		// mounted to a container) - this is because it may be saved as a
		// resource cache.
		//
		// we use a unique mount path used as a search criteria in
		// findOrCreateVolumeForContainer so we can distinguish between
		// streamed-in volumes and mounted volumes.
		streamedVolume, err := worker.findOrCreateVolumeForStreaming(
			ctx,
			privileged,
			container,
			spec.TeamID,
			input.mountPath,
			io.Discard,
		)
		if err != nil {
			return nil, err
		}

		mounts[i] = mountableLocalInput{
			cowParent: streamedVolume,
			mountPath: input.mountPath,
		}

		g.Go(func() error {
			return worker.streamer.Stream(groupCtx, input.volume, streamedVolume)
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	logger.Debug("streamed-non-local-volumes", lager.Data{"volumes-streamed": len(inputs)})

	return mounts, nil
}

func (worker *Worker) createOutputVolumes(
	ctx context.Context,
	spec runtime.ContainerSpec,
	privileged bool,
	container db.CreatingContainer,
	inputDestinationPaths map[string]bool,
) ([]runtime.VolumeMount, error) {
	var mounts []runtime.VolumeMount

	for _, outputPath := range spec.Outputs {
		cleanedOutputPath := filepath.Clean(outputPath)

		// reuse volume if output path is the same as input
		if inputDestinationPaths[cleanedOutputPath] {
			continue
		}

		outVolume, err := worker.findOrCreateVolumeForContainer(
			ctx,
			baggageclaim.VolumeSpec{
				Strategy:   baggageclaim.EmptyStrategy{},
				Privileged: privileged,
			},
			container,
			spec.TeamID,
			cleanedOutputPath,
			io.Discard,
		)
		if err != nil {
			return nil, err
		}

		mounts = append(mounts, runtime.VolumeMount{
			Volume:    outVolume,
			MountPath: cleanedOutputPath,
		})
	}

	return mounts, nil
}

func (worker *Worker) cloneCacheVolumes(
	ctx context.Context,
	spec runtime.ContainerSpec,
	privileged bool,
	container db.CreatingContainer,
) ([]runtime.VolumeMount, error) {
	mounts := make([]runtime.VolumeMount, len(spec.Caches))

	for i, cachePath := range spec.Caches {
		cachePath = filepath.Clean(cachePath)

		// TODO: skip over cache if path already used?
		volume, found, err := worker.findVolumeForTaskCache(ctx, spec.TeamID, spec.JobID, spec.StepName, cachePath)
		if err != nil {
			return nil, err
		}

		mountPath := cachePath
		if !filepath.IsAbs(cachePath) {
			mountPath = filepath.Join(spec.Dir, cachePath)
		}

		var mountedVolume Volume
		if found {
			// create COW volumes for caches in case multiple builds are
			// running with the same cache
			mountedVolume, err = worker.findOrCreateCOWVolumeForContainer(
				ctx,
				privileged,
				container,
				volume,
				spec.TeamID,
				mountPath,
				io.Discard,
			)
			if err != nil {
				return nil, err
			}
		} else {
			// create empty volumes for caches that are not present on the
			// host. these will become the new base cache volume for future
			// builds
			mountedVolume, err = worker.findOrCreateVolumeForContainer(
				ctx,
				baggageclaim.VolumeSpec{
					Strategy:   baggageclaim.EmptyStrategy{},
					Privileged: privileged,
				},
				container,
				spec.TeamID,
				mountPath,
				io.Discard,
			)
			if err != nil {
				return nil, err
			}
		}

		mounts[i] = runtime.VolumeMount{
			Volume:    mountedVolume,
			MountPath: mountPath,
		}
	}

	return mounts, nil
}

func (worker *Worker) getBindMounts(ctx context.Context, volumeMounts []runtime.VolumeMount, spec runtime.ContainerSpec) ([]garden.BindMount, error) {
	var bindMounts []garden.BindMount

	for _, volumeMount := range volumeMounts {
		bindMounts = append(bindMounts, garden.BindMount{
			SrcPath: volumeMount.Volume.(Volume).Path(),
			DstPath: volumeMount.MountPath,
			Mode:    garden.BindMountModeRW,
		})
	}

	if spec.CertsBindMount {
		certsVolume, found, err := worker.findOrCreateVolumeForResourceCerts(lagerctx.NewContext(ctx, lagerctx.FromContext(ctx).Session("worker-certs-volume")))
		if err != nil {
			return nil, err
		}
		if found {
			bindMounts = append(bindMounts, garden.BindMount{
				SrcPath: certsVolume.Path(),
				DstPath: "/etc/ssl/certs",
				Mode:    garden.BindMountModeRO,
			})
		}
	}

	return bindMounts, nil
}

func anyMountTo(path string, volumeMounts []runtime.VolumeMount) bool {
	for _, mnt := range volumeMounts {
		if filepath.Clean(mnt.MountPath) == filepath.Clean(path) {
			return true
		}
	}

	return false
}

func toGardenLimits(cl runtime.ContainerLimits) garden.Limits {
	const gardenLimitDefault = uint64(0)

	gardenLimits := garden.Limits{}
	if cl.CPU == nil {
		gardenLimits.CPU = garden.CPULimits{LimitInShares: gardenLimitDefault}
	} else {
		gardenLimits.CPU = garden.CPULimits{LimitInShares: *cl.CPU}
	}
	if cl.Memory == nil {
		gardenLimits.Memory = garden.MemoryLimits{LimitInBytes: gardenLimitDefault}
	} else {
		gardenLimits.Memory = garden.MemoryLimits{LimitInBytes: *cl.Memory}
	}
	return gardenLimits
}

func markContainerAsFailed(logger lager.Logger, container db.CreatingContainer) {
	_, err := container.Failed()
	if err != nil {
		logger.Error("failed-to-mark-container-as-failed", err)
	}
	metric.Metrics.FailedContainers.Inc()
}

func (worker *Worker) DBWorker() db.Worker {
	return worker.dbWorker
}

// For testing
func (worker *Worker) GardenClient() gclient.Client {
	return worker.gardenClient
}
func (worker *Worker) BaggageclaimClient() baggageclaim.Client {
	return worker.bcClient
}
