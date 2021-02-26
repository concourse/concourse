package gardenruntime

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker/gclient"
	"github.com/concourse/concourse/tracing"
	"golang.org/x/sync/errgroup"
)

type Pool interface {
	LocateVolume(logger lager.Logger, teamID int, handle string) (runtime.Volume, runtime.Worker, bool, error)
}

type Streamer interface {
	Stream(ctx context.Context, srcWorker string, src runtime.Volume, dst runtime.Volume) error
	StreamFile(ctx context.Context, src runtime.Volume, path string) (io.ReadCloser, error)
}

type Worker struct {
	pool     Pool
	streamer Streamer

	dbWorker     db.Worker
	gardenClient gclient.Client
	bcClient     baggageclaim.Client

	db DB
}

type DB struct {
	VolumeRepo                    db.VolumeRepository
	TaskCacheFactory              db.TaskCacheFactory
	WorkerBaseResourceTypeFactory db.WorkerBaseResourceTypeFactory
	LockFactory                   lock.LockFactory
}

func NewWorker(dbWorker db.Worker, gardenClient gclient.Client, bcClient baggageclaim.Client, db DB, pool Pool, streamer Streamer) *Worker {
	return &Worker{
		pool:     pool,
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
) (runtime.Container, error) {
	logger := lagerctx.FromContext(ctx)
	c, err := worker.findOrCreateContainer(ctx, logger, owner, metadata, containerSpec)
	if err != nil {
		return c, fmt.Errorf("find or create container on worker %s: %w", worker.Name(), err)
	}
	return c, err
}

func (worker *Worker) findOrCreateContainer(
	ctx context.Context,
	logger lager.Logger,
	owner db.ContainerOwner,
	metadata db.ContainerMetadata,
	containerSpec runtime.ContainerSpec,
) (runtime.Container, error) {
	var (
		gardenContainer   gclient.Container
		createdContainer  db.CreatedContainer
		creatingContainer db.CreatingContainer
		containerHandle   string
		err               error
	)

	creatingContainer, createdContainer, err = worker.dbWorker.FindContainer(owner)
	if err != nil {
		return nil, fmt.Errorf("find in db: %w", err)
	}

	// ensure either creatingContainer or createdContainer exists
	if creatingContainer != nil {
		containerHandle = creatingContainer.Handle()
	} else if createdContainer != nil {
		containerHandle = createdContainer.Handle()
	} else {
		logger.Debug("creating-container-in-db")
		creatingContainer, err = worker.dbWorker.CreateContainer(
			owner,
			metadata,
		)
		if err != nil {
			logger.Error("failed-to-create-container-in-db", err)
			if _, ok := err.(db.ContainerOwnerDisappearedError); ok {
				return nil, ErrResourceConfigCheckSessionExpired
			}

			return nil, fmt.Errorf("create container: %w", err)
		}
		logger.Debug("created-creating-container-in-db")
		containerHandle = creatingContainer.Handle()
	}

	logger = logger.WithData(lager.Data{"container": containerHandle})

	gardenContainer, err = worker.gardenClient.Lookup(containerHandle)
	if err != nil {
		if _, ok := err.(garden.ContainerNotFoundError); !ok {
			logger.Error("failed-to-lookup-creating-container-in-garden", err)
			return nil, err
		}
	}

	// if createdContainer exists, gardenContainer should also exist
	if createdContainer != nil {
		logger = logger.WithData(lager.Data{"container": containerHandle})
		logger.Debug("found-created-container-in-db")

		if gardenContainer == nil {
			return nil, garden.ContainerNotFoundError{Handle: containerHandle}
		}
		return worker.constructContainer(
			logger,
			createdContainer,
			gardenContainer,
		)
	}

	// we now have a creatingContainer. If a gardenContainer does not exist, we
	// will create one. If it does exist, we will transition the creatingContainer
	// to created and return a worker.Container
	if gardenContainer == nil {
		gardenContainer, err = worker.createGardenContainer(ctx, logger, containerSpec, creatingContainer)
		if err != nil {
			logger.Error("failed-to-create-container-in-garden", err)
			markContainerAsFailed(logger, creatingContainer)
			return nil, err
		}
	}

	logger.Debug("created-container-in-garden")

	createdContainer, err = creatingContainer.Created()
	if err != nil {
		logger.Error("failed-to-mark-container-as-created", err)
		_ = worker.gardenClient.Destroy(containerHandle)
		return nil, err
	}

	logger.Debug("created-container-in-db")
	metric.Metrics.ContainersCreated.Inc()

	return worker.constructContainer(
		logger,
		createdContainer,
		gardenContainer,
	)
}

func (worker *Worker) createGardenContainer(
	ctx context.Context,
	logger lager.Logger,
	containerSpec runtime.ContainerSpec,
	creatingContainer db.CreatingContainer,
) (gclient.Container, error) {
	fetchedImage, err := worker.fetchImageForContainer(
		ctx,
		logger,
		containerSpec.ImageSpec,
		containerSpec.TeamID,
		creatingContainer,
	)
	if err != nil {
		logger.Error("failed-to-fetch-image-for-container", err)
		markContainerAsFailed(logger, creatingContainer)
		return nil, err
	}

	volumeMounts, err := worker.createVolumes(ctx, logger, fetchedImage.Privileged, creatingContainer, containerSpec)
	if err != nil {
		logger.Error("failed-to-create-volume-mounts-for-container", err)
		markContainerAsFailed(logger, creatingContainer)
		return nil, err
	}

	bindMounts, err := worker.getBindMounts(logger, volumeMounts, containerSpec)
	if err != nil {
		logger.Error("failed-to-create-bind-mounts-for-container", err)
		markContainerAsFailed(logger, creatingContainer)
		return nil, err
	}

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
	logger lager.Logger,
	createdContainer db.CreatedContainer,
	gardenContainer gclient.Container,
) (Container, error) {
	logger = logger.WithData(
		lager.Data{
			"container": createdContainer.Handle(),
			"worker":    worker.Name(),
		},
	)

	createdVolumes, err := worker.db.VolumeRepo.FindVolumesForContainer(createdContainer)
	if err != nil {
		logger.Error("failed-to-find-container-volumes", err)
		return Container{}, err
	}

	volumeMounts := make([]runtime.VolumeMount, len(createdVolumes))

	for i, dbVolume := range createdVolumes {
		volumeLogger := logger.Session("volume", lager.Data{
			"handle": dbVolume.Handle(),
		})

		volume, found, err := worker.LookupVolume(logger, dbVolume.Handle())
		if err != nil {
			volumeLogger.Error("failed-to-lookup-volume", err)
			return Container{}, err
		}

		if !found {
			err := MountedVolumeMissingFromWorker{
				Handle:     dbVolume.Handle(),
				WorkerName: worker.Name(),
			}
			volumeLogger.Error("volume-is-missing-on-worker", err, lager.Data{"handle": dbVolume.Handle()})
			return Container{}, err
		}

		volumeMounts[i] = runtime.VolumeMount{
			Volume:    volume.(Volume),
			MountPath: dbVolume.Path(),
		}
	}
	return Container{
		gardenContainer: gardenContainer,
		dbContainer:     createdContainer,
		volumeMounts:    volumeMounts,
	}, nil
}

// creates volumes required to run any step:
// * scratch
// * working dir (i.e. spec.Dir)
// * inputs
// * outputs
// * caches (COW if exists, empty otherwise)
func (worker *Worker) createVolumes(
	ctx context.Context,
	logger lager.Logger,
	privileged bool,
	creatingContainer db.CreatingContainer,
	spec runtime.ContainerSpec,
) ([]runtime.VolumeMount, error) {
	var volumeMounts []runtime.VolumeMount

	scratchVolume, err := worker.findOrCreateVolumeForContainer(
		logger,
		runtime.VolumeSpec{
			Strategy:   baggageclaim.EmptyStrategy{},
			Privileged: privileged,
		},
		creatingContainer,
		spec.TeamID,
		"/scratch",
	)
	if err != nil {
		return nil, err
	}

	scratchMount := runtime.VolumeMount{
		Volume:    scratchVolume,
		MountPath: "/scratch",
	}

	volumeMounts = append(volumeMounts, scratchMount)

	inputVolumeMounts, inputDestinationPaths, err := worker.cloneInputVolumes(ctx, logger, spec, privileged, creatingContainer)
	if err != nil {
		return nil, err
	}

	outputVolumeMounts, err := worker.createOutputVolumes(logger, spec, privileged, creatingContainer, inputDestinationPaths)
	if err != nil {
		return nil, err
	}

	cacheVolumeMounts, err := worker.cloneCacheVolumes(logger, spec, privileged, creatingContainer)
	if err != nil {
		return nil, err
	}

	ioVolumeMounts := append(inputVolumeMounts, outputVolumeMounts...)
	ioVolumeMounts = append(ioVolumeMounts, cacheVolumeMounts...)

	sort.Sort(byMountPath(ioVolumeMounts))

	volumeMounts = append(volumeMounts, ioVolumeMounts...)

	// if the working dir is already mounted, we can just re-use that volume.
	// otherwise, we must create a new empty volume
	if !anyMountTo(spec.Dir, volumeMounts) {
		workdirVolume, err := worker.findOrCreateVolumeForContainer(
			logger,
			runtime.VolumeSpec{
				Strategy:   baggageclaim.EmptyStrategy{},
				Privileged: privileged,
			},
			creatingContainer,
			spec.TeamID,
			spec.Dir,
		)
		if err != nil {
			return nil, err
		}

		volumeMounts = append(volumeMounts, runtime.VolumeMount{
			Volume:    workdirVolume,
			MountPath: spec.Dir,
		})
	}

	return volumeMounts, nil
}

type mountableLocalInput struct {
	cowParent Volume
	mountPath string
}

type mountableRemoteInput struct {
	volume    runtime.Volume
	srcWorker string
	mountPath string
}

func (worker *Worker) cloneInputVolumes(
	ctx context.Context,
	logger lager.Logger,
	spec runtime.ContainerSpec,
	privileged bool,
	container db.CreatingContainer,
) ([]runtime.VolumeMount, map[string]bool, error) {
	inputDestinationPaths := make(map[string]bool)

	var localInputs []mountableLocalInput
	var remoteInputs []mountableRemoteInput

	for _, input := range spec.Inputs {
		volume, otherWorker, found, err := worker.pool.LocateVolume(logger, spec.TeamID, input.VolumeHandle)
		if err != nil {
			return nil, nil, err
		}
		if !found {
			return nil, nil, InputNotFoundError{Input: input}
		}

		cleanedInputPath := filepath.Clean(input.DestinationPath)
		inputDestinationPaths[cleanedInputPath] = true

		if worker.Name() == otherWorker.Name() {
			localInputs = append(localInputs, mountableLocalInput{
				cowParent: volume.(Volume),
				mountPath: input.DestinationPath,
			})
		} else {
			remoteInputs = append(remoteInputs, mountableRemoteInput{
				volume:    volume,
				srcWorker: otherWorker.Name(),
				mountPath: input.DestinationPath,
			})
		}
	}

	mounts := make([]runtime.VolumeMount, 0, len(localInputs)+len(remoteInputs))

	localMounts, err := worker.cloneLocalInputVolumes(logger, spec, privileged, container, localInputs)
	if err != nil {
		return nil, nil, err
	}
	mounts = append(mounts, localMounts...)

	remoteMounts, err := worker.cloneRemoteInputVolumes(ctx, logger, spec, privileged, container, remoteInputs)
	if err != nil {
		return nil, nil, err
	}
	mounts = append(mounts, remoteMounts...)

	return mounts, inputDestinationPaths, nil
}

func (worker *Worker) cloneLocalInputVolumes(
	logger lager.Logger,
	spec runtime.ContainerSpec,
	privileged bool,
	container db.CreatingContainer,
	inputs []mountableLocalInput,
) ([]runtime.VolumeMount, error) {
	mounts := make([]runtime.VolumeMount, len(inputs))

	for i, input := range inputs {
		cowVolume, err := worker.findOrCreateCOWVolumeForContainer(
			logger,
			privileged,
			container,
			input.cowParent,
			spec.TeamID,
			input.mountPath,
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

func (worker *Worker) cloneRemoteInputVolumes(
	ctx context.Context,
	logger lager.Logger,
	spec runtime.ContainerSpec,
	privileged bool,
	container db.CreatingContainer,
	inputs []mountableRemoteInput,
) ([]runtime.VolumeMount, error) {
	if len(inputs) == 0 {
		return []runtime.VolumeMount{}, nil
	}
	mounts := make([]runtime.VolumeMount, len(inputs))

	ctx, span := tracing.StartSpan(ctx, "worker.cloneRemoteVolumes", tracing.Attrs{"container_id": container.Handle()})
	defer span.End()

	g, groupCtx := errgroup.WithContext(ctx)

	for i, input := range inputs {
		// capture loop vars so each goroutine gets its own copy
		i, input := i, input

		inputVolume, err := worker.findOrCreateVolumeForContainer(
			logger,
			runtime.VolumeSpec{
				Strategy:   baggageclaim.EmptyStrategy{},
				Privileged: privileged,
			},
			container,
			spec.TeamID,
			input.mountPath,
		)
		if err != nil {
			return nil, err
		}

		mounts[i] = runtime.VolumeMount{
			Volume:    inputVolume,
			MountPath: input.mountPath,
		}

		g.Go(func() error {
			return worker.streamer.Stream(groupCtx, input.srcWorker, input.volume, inputVolume)
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	logger.Debug("streamed-non-local-volumes", lager.Data{"volumes-streamed": len(inputs)})

	return mounts, nil
}

func (worker *Worker) createOutputVolumes(
	logger lager.Logger,
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
			logger,
			runtime.VolumeSpec{
				Strategy:   baggageclaim.EmptyStrategy{},
				Privileged: privileged,
			},
			container,
			spec.TeamID,
			cleanedOutputPath,
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
	logger lager.Logger,
	spec runtime.ContainerSpec,
	privileged bool,
	container db.CreatingContainer,
) ([]runtime.VolumeMount, error) {
	mounts := make([]runtime.VolumeMount, len(spec.Caches))

	for i, cachePath := range spec.Caches {
		// TODO: skip over cache if path already used?
		volume, found, err := worker.findVolumeForTaskCache(logger, spec.TeamID, spec.JobID, spec.StepName, cachePath)
		if err != nil {
			return nil, err
		}

		var mountedVolume Volume
		if found {
			// create COW volumes for caches in case multiple builds are
			// running with the same cache
			mountedVolume, err = worker.findOrCreateCOWVolumeForContainer(
				logger,
				privileged,
				container,
				volume,
				spec.TeamID,
				cachePath,
			)
			if err != nil {
				return nil, err
			}
		} else {
			// create empty volumes for caches that are not present on the
			// host. these will become the new base cache volume for future
			// builds
			mountedVolume, err = worker.findOrCreateVolumeForContainer(
				logger,
				runtime.VolumeSpec{
					Strategy:   baggageclaim.EmptyStrategy{},
					Privileged: privileged,
				},
				container,
				spec.TeamID,
				cachePath,
			)
			if err != nil {
				return nil, err
			}
		}

		mounts[i] = runtime.VolumeMount{
			MountPath: cachePath,
			Volume:    mountedVolume,
		}
	}

	return mounts, nil
}

func (worker *Worker) getBindMounts(logger lager.Logger, volumeMounts []runtime.VolumeMount, spec runtime.ContainerSpec) ([]garden.BindMount, error) {
	var bindMounts []garden.BindMount

	for _, volumeMount := range volumeMounts {
		bindMounts = append(bindMounts, garden.BindMount{
			SrcPath: volumeMount.Volume.(Volume).Path(),
			DstPath: volumeMount.MountPath,
			Mode:    garden.BindMountModeRW,
		})
	}

	if spec.CertsBindMount {
		certsVolume, found, err := worker.findOrCreateVolumeForResourceCerts(logger.Session("worker-certs-volume"))
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

// For testing
func (worker *Worker) GardenClient() gclient.Client {
	return worker.gardenClient
}
func (worker *Worker) BaggageclaimClient() baggageclaim.Client {
	return worker.bcClient
}
