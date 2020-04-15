package worker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"path"
	"strconv"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/hashicorp/go-multierror"
)

const taskProcessID = "task"
const taskExitStatusPropertyName = "concourse:exit-status"

//go:generate counterfeiter . Client

type Client interface {
	FindContainer(logger lager.Logger, teamID int, handle string) (Container, bool, error)
	FindVolume(logger lager.Logger, teamID int, handle string) (Volume, bool, error)
	CreateVolume(logger lager.Logger, vSpec VolumeSpec, wSpec WorkerSpec, volumeType db.VolumeType) (Volume, error)
	StreamFileFromArtifact(
		ctx context.Context,
		logger lager.Logger,
		artifact runtime.Artifact,
		filePath string,
	) (io.ReadCloser, error)

	RunCheckStep(
		ctx context.Context,
		logger lager.Logger,
		owner db.ContainerOwner,
		containerSpec ContainerSpec,
		workerSpec WorkerSpec,
		strategy ContainerPlacementStrategy,
		containerMetadata db.ContainerMetadata,
		resourceTypes atc.VersionedResourceTypes,
		timeout time.Duration,
		checkable resource.Resource,
	) (CheckResult, error)

	RunTaskStep(
		context.Context,
		lager.Logger,
		db.ContainerOwner,
		ContainerSpec,
		WorkerSpec,
		ContainerPlacementStrategy,
		db.ContainerMetadata,
		ImageFetcherSpec,
		runtime.ProcessSpec,
		runtime.StartingEventDelegate,
		lock.LockFactory,
	) (TaskResult, error)

	RunPutStep(
		context.Context,
		lager.Logger,
		db.ContainerOwner,
		ContainerSpec,
		WorkerSpec,
		ContainerPlacementStrategy,
		db.ContainerMetadata,
		ImageFetcherSpec,
		runtime.ProcessSpec,
		runtime.StartingEventDelegate,
		resource.Resource,
	) (PutResult, error)

	RunGetStep(
		context.Context,
		lager.Logger,
		db.ContainerOwner,
		ContainerSpec,
		WorkerSpec,
		ContainerPlacementStrategy,
		db.ContainerMetadata,
		ImageFetcherSpec,
		runtime.ProcessSpec,
		runtime.StartingEventDelegate,
		db.UsedResourceCache,
		resource.Resource,
	) (GetResult, error)
}

func NewClient(pool Pool, provider WorkerProvider, compression compression.Compression) *client {
	return &client{
		pool:        pool,
		provider:    provider,
		compression: compression,
	}
}

type client struct {
	pool        Pool
	provider    WorkerProvider
	compression compression.Compression
}

type TaskResult struct {
	ExitStatus   int
	VolumeMounts []VolumeMount
}

type CheckResult struct {
	Versions []atc.Version
}

type PutResult struct {
	ExitStatus    int
	VersionResult runtime.VersionResult
}

type GetResult struct {
	ExitStatus    int
	VersionResult runtime.VersionResult
	GetArtifact   runtime.GetArtifact
}

type ImageFetcherSpec struct {
	ResourceTypes atc.VersionedResourceTypes
	Delegate      ImageFetchingDelegate
}

func (client *client) FindContainer(logger lager.Logger, teamID int, handle string) (Container, bool, error) {
	worker, found, err := client.provider.FindWorkerForContainer(
		logger.Session("find-worker"),
		teamID,
		handle,
	)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return worker.FindContainerByHandle(logger, teamID, handle)
}

func (client *client) FindVolume(logger lager.Logger, teamID int, handle string) (Volume, bool, error) {
	worker, found, err := client.provider.FindWorkerForVolume(
		logger.Session("find-worker"),
		teamID,
		handle,
	)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return worker.LookupVolume(logger, handle)
}

func (client *client) CreateVolume(logger lager.Logger, volumeSpec VolumeSpec, workerSpec WorkerSpec, volumeType db.VolumeType) (Volume, error) {
	worker, err := client.pool.FindOrChooseWorker(logger, workerSpec)
	if err != nil {
		return nil, err
	}

	return worker.CreateVolume(logger, volumeSpec, workerSpec.TeamID, volumeType)
}

var checkProcessSpec = runtime.ProcessSpec{
	Path: "/opt/resource/check",
}

func (client *client) RunCheckStep(
	ctx context.Context,
	logger lager.Logger,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	workerSpec WorkerSpec,
	strategy ContainerPlacementStrategy,
	containerMetadata db.ContainerMetadata,
	resourceTypes atc.VersionedResourceTypes,
	timeout time.Duration,
	checkable resource.Resource,
) (CheckResult, error) {
	chosenWorker, err := client.pool.FindOrChooseWorkerForContainer(
		ctx,
		logger,
		owner,
		containerSpec,
		workerSpec,
		strategy,
	)
	if err != nil {
		return CheckResult{}, fmt.Errorf("find or choose worker for container: %w", err)
	}

	container, err := chosenWorker.FindOrCreateContainer(
		ctx,
		logger,
		&NoopImageFetchingDelegate{},
		owner,
		containerMetadata,
		containerSpec,
		resourceTypes,
	)
	if err != nil {
		return CheckResult{}, fmt.Errorf("find or create container: %w", err)
	}

	deadline, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	versions, err := checkable.Check(deadline, checkProcessSpec, container)
	if err != nil {
		if err == context.DeadlineExceeded {
			return CheckResult{}, fmt.Errorf("timed out after %v checking for new versions", timeout)
		}

		return CheckResult{}, fmt.Errorf("check: %w", err)
	}

	return CheckResult{Versions: versions}, nil
}

func (client *client) RunTaskStep(
	ctx context.Context,
	logger lager.Logger,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	workerSpec WorkerSpec,
	strategy ContainerPlacementStrategy,
	metadata db.ContainerMetadata,
	imageFetcherSpec ImageFetcherSpec,
	processSpec runtime.ProcessSpec,
	eventDelegate runtime.StartingEventDelegate,
	lockFactory lock.LockFactory,
) (TaskResult, error) {
	err := client.wireInputsAndCaches(logger, &containerSpec)
	if err != nil {
		return TaskResult{}, err
	}

	if containerSpec.ImageSpec.ImageArtifact != nil {
		err = client.wireImageVolume(logger, &containerSpec.ImageSpec)
		if err != nil {
			return TaskResult{}, err
		}
	}

	chosenWorker, err := client.chooseTaskWorker(
		ctx,
		logger,
		strategy,
		lockFactory,
		owner,
		containerSpec,
		workerSpec,
		processSpec.StdoutWriter,
	)
	if err != nil {
		return TaskResult{}, err
	}

	if strategy.ModifiesActiveTasks() {
		defer decreaseActiveTasks(logger.Session("decrease-active-tasks"), chosenWorker)
	}

	container, err := chosenWorker.FindOrCreateContainer(
		ctx,
		logger,
		imageFetcherSpec.Delegate,
		owner,
		metadata,
		containerSpec,
		imageFetcherSpec.ResourceTypes,
	)

	if err != nil {
		return TaskResult{}, err
	}

	// container already exited
	exitStatusProp, _ := container.Properties()
	code := exitStatusProp[taskExitStatusPropertyName]
	if code != "" {
		logger.Info("already-exited", lager.Data{"status": taskExitStatusPropertyName})

		status, err := strconv.Atoi(code)
		if err != nil {
			return TaskResult{}, err
		}

		return TaskResult{
			ExitStatus:   status,
			VolumeMounts: container.VolumeMounts(),
		}, err
	}

	processIO := garden.ProcessIO{
		Stdout: processSpec.StdoutWriter,
		Stderr: processSpec.StderrWriter,
	}

	process, err := container.Attach(context.Background(), taskProcessID, processIO)
	if err == nil {
		logger.Info("already-running")
	} else {
		eventDelegate.Starting(logger)
		logger.Info("spawning")

		process, err = container.Run(
			context.Background(),
			garden.ProcessSpec{
				ID: taskProcessID,

				Path: processSpec.Path,
				Args: processSpec.Args,

				Dir: path.Join(metadata.WorkingDirectory, processSpec.Dir),

				// Guardian sets the default TTY window size to width: 80, height: 24,
				// which creates ANSI control sequences that do not work with other window sizes
				TTY: &garden.TTYSpec{
					WindowSize: &garden.WindowSize{Columns: 500, Rows: 500},
				},
			},
			processIO,
		)

		if err != nil {
			return TaskResult{}, err
		}
	}

	logger.Info("attached")

	exitStatusChan := make(chan processStatus)

	go func() {
		status := processStatus{}
		status.processStatus, status.processErr = process.Wait()
		exitStatusChan <- status
	}()

	select {
	case <-ctx.Done():
		err = container.Stop(false)
		if err != nil {
			logger.Error("stopping-container", err)
		}

		status := <-exitStatusChan
		return TaskResult{
			ExitStatus:   status.processStatus,
			VolumeMounts: container.VolumeMounts(),
		}, ctx.Err()

	case status := <-exitStatusChan:
		if status.processErr != nil {
			return TaskResult{
				ExitStatus: status.processStatus,
			}, status.processErr
		}

		err = container.SetProperty(taskExitStatusPropertyName, fmt.Sprintf("%d", status.processStatus))
		if err != nil {
			return TaskResult{
				ExitStatus: status.processStatus,
			}, err
		}
		return TaskResult{
			ExitStatus:   status.processStatus,
			VolumeMounts: container.VolumeMounts(),
		}, err
	}
}

func (client *client) RunGetStep(
	ctx context.Context,
	logger lager.Logger,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	workerSpec WorkerSpec,
	strategy ContainerPlacementStrategy,
	containerMetadata db.ContainerMetadata,
	imageFetcherSpec ImageFetcherSpec,
	processSpec runtime.ProcessSpec,
	eventDelegate runtime.StartingEventDelegate,
	resourceCache db.UsedResourceCache,
	resource resource.Resource,
) (GetResult, error) {

	chosenWorker, err := client.pool.FindOrChooseWorkerForContainer(
		ctx,
		logger,
		owner,
		containerSpec,
		workerSpec,
		strategy,
	)
	if err != nil {
		return GetResult{}, err
	}

	sign, err := resource.Signature()
	if err != nil {
		return GetResult{}, err
	}

	lockName := lockName(sign, chosenWorker.Name())

	// TODO: this needs to be emitted right before executing the `in` script
	eventDelegate.Starting(logger)

	getResult, _, err := chosenWorker.Fetch(
		ctx,
		logger,
		containerMetadata,
		chosenWorker,
		containerSpec,
		processSpec,
		resource,
		owner,
		imageFetcherSpec,
		resourceCache,
		lockName,
	)
	return getResult, err
}

func (client *client) chooseTaskWorker(
	ctx context.Context,
	logger lager.Logger,
	strategy ContainerPlacementStrategy,
	lockFactory lock.LockFactory,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	workerSpec WorkerSpec,
	outputWriter io.Writer,
) (Worker, error) {
	var (
		chosenWorker      Worker
		activeTasksLock   lock.Lock
		elapsed           time.Duration
		err               error
		existingContainer bool
	)

	for {
		if strategy.ModifiesActiveTasks() {
			var acquired bool
			activeTasksLock, acquired, err = lockFactory.Acquire(logger, lock.NewActiveTasksLockID())
			if err != nil {
				return nil, err
			}

			if !acquired {
				time.Sleep(time.Second)
				continue
			}
			existingContainer, err = client.pool.ContainerInWorker(logger, owner, containerSpec, workerSpec)
			if err != nil {
				release_err := activeTasksLock.Release()
				if release_err != nil {
					err = multierror.Append(err, release_err)
				}
				return nil, err
			}
		}

		chosenWorker, err = client.pool.FindOrChooseWorkerForContainer(
			ctx,
			logger,
			owner,
			containerSpec,
			workerSpec,
			strategy,
		)
		if err != nil {
			return nil, err
		}

		if strategy.ModifiesActiveTasks() {
			waitWorker := time.Duration(5 * time.Second) // Workers polling frequency

			select {
			case <-ctx.Done():
				logger.Info("aborted-waiting-worker")
				err = activeTasksLock.Release()
				if err != nil {
					return nil, err
				}
				return nil, ctx.Err()
			default:
			}

			if chosenWorker == nil {
				err = activeTasksLock.Release()
				if err != nil {
					return nil, err
				}

				if elapsed == 0 {
					// first round waiting...
					metric.TasksWaiting.Inc()
					defer metric.TasksWaiting.Dec()
				}

				if elapsed%time.Duration(time.Minute) == 0 { // Every minute report that it is still waiting

					_, err := outputWriter.Write([]byte("All workers are busy at the moment, please stand-by.\n"))
					if err != nil {
						logger.Error("failed-to-report-status", err)
					}
				}

				elapsed += waitWorker
				time.Sleep(waitWorker)
				continue
			}

			if !existingContainer {
				err = chosenWorker.IncreaseActiveTasks()
				if err != nil {
					logger.Error("failed-to-increase-active-tasks", err)
				}
			}

			err = activeTasksLock.Release()
			if err != nil {
				return nil, err
			}

			if elapsed > 0 {
				_, err := outputWriter.Write([]byte(fmt.Sprintf("Found a free worker after waiting %s.\n", elapsed)))
				if err != nil {
					logger.Error("failed-to-report-status", err)
				}
			}
		}

		break
	}

	return chosenWorker, nil
}

func decreaseActiveTasks(logger lager.Logger, w Worker) {
	err := w.DecreaseActiveTasks()
	if err != nil {
		logger.Error("failed-to-decrease-active-tasks", err)
		return
	}
}

type processStatus struct {
	processStatus int
	processErr    error
}

func (client *client) RunPutStep(
	ctx context.Context,
	logger lager.Logger,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	workerSpec WorkerSpec,
	strategy ContainerPlacementStrategy,
	metadata db.ContainerMetadata,
	imageFetcherSpec ImageFetcherSpec,
	spec runtime.ProcessSpec,
	eventDelegate runtime.StartingEventDelegate,
	resource resource.Resource,
) (PutResult, error) {

	vr := runtime.VersionResult{}
	err := client.wireInputsAndCaches(logger, &containerSpec)
	if err != nil {
		return PutResult{}, err
	}

	chosenWorker, err := client.pool.FindOrChooseWorkerForContainer(
		ctx,
		logger,
		owner,
		containerSpec,
		workerSpec,
		strategy,
	)
	if err != nil {
		return PutResult{}, err
	}

	container, err := chosenWorker.FindOrCreateContainer(
		ctx,
		logger,
		imageFetcherSpec.Delegate,
		owner,
		metadata,
		containerSpec,
		imageFetcherSpec.ResourceTypes,
	)
	if err != nil {
		return PutResult{}, err
	}

	// container already exited
	exitStatusProp, err := container.Property(taskExitStatusPropertyName)
	if err == nil {
		logger.Info("already-exited", lager.Data{"status": exitStatusProp})

		status, err := strconv.Atoi(exitStatusProp)
		if err != nil {
			return PutResult{}, err
		}

		return PutResult{
			ExitStatus:    status,
			VersionResult: runtime.VersionResult{},
		}, nil
	}

	eventDelegate.Starting(logger)

	vr, err = resource.Put(ctx, spec, container)
	if err != nil {
		if failErr, ok := err.(runtime.ErrResourceScriptFailed); ok {
			return PutResult{
				ExitStatus:    failErr.ExitStatus,
				VersionResult: runtime.VersionResult{},
			}, nil
		} else {
			return PutResult{}, err
		}
	}
	return PutResult{
		ExitStatus:    0,
		VersionResult: vr,
	}, nil
}

// TODO (runtime) don't modify spec inside here, Specs don't change after you write them
func (client *client) wireInputsAndCaches(logger lager.Logger, spec *ContainerSpec) error {
	var inputs []InputSource

	for path, artifact := range spec.ArtifactByPath {

		if cache, ok := artifact.(*runtime.CacheArtifact); ok {
			// task caches may not have a volume, it will be discovered on
			// the worker later. We do not stream task caches
			source := NewCacheArtifactSource(*cache)
			inputs = append(inputs, inputSource{source, path})
		} else {
			artifactVolume, found, err := client.FindVolume(logger, spec.TeamID, artifact.ID())
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("volume not found for artifact id %v type %T", artifact.ID(), artifact)
			}

			source := NewStreamableArtifactSource(artifact, artifactVolume, client.compression)
			inputs = append(inputs, inputSource{source, path})
		}
	}

	spec.Inputs = inputs
	return nil
}

func (client *client) wireImageVolume(logger lager.Logger, spec *ImageSpec) error {

	imageArtifact := spec.ImageArtifact

	artifactVolume, found, err := client.FindVolume(logger, 0, imageArtifact.ID())
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("volume not found for artifact id %v type %T", imageArtifact.ID(), imageArtifact)
	}

	spec.ImageArtifactSource = NewStreamableArtifactSource(imageArtifact, artifactVolume, client.compression)

	return nil
}

func (client *client) StreamFileFromArtifact(
	ctx context.Context,
	logger lager.Logger,
	artifact runtime.Artifact,
	filePath string,
) (io.ReadCloser, error) {
	artifactVolume, found, err := client.FindVolume(logger, 0, artifact.ID())
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, baggageclaim.ErrVolumeNotFound
	}

	source := artifactSource{
		artifact:    artifact,
		volume:      artifactVolume,
		compression: client.compression,
	}
	return source.StreamFile(ctx, logger, filePath)
}

func lockName(resourceJSON []byte, workerName string) string {
	jsonRes := append(resourceJSON, []byte(workerName)...)
	return fmt.Sprintf("%x", sha256.Sum256(jsonRes))
}
