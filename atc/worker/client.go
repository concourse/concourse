package worker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
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
		context.Context,
		lager.Logger,
		db.ContainerOwner,
		ContainerSpec,
		WorkerSpec,
		ContainerPlacementStrategy,
		db.ContainerMetadata,
		runtime.ProcessSpec,
		runtime.StartingEventDelegate,
		resource.Resource,
	) (CheckResult, error)

	RunTaskStep(
		context.Context,
		lager.Logger,
		db.ContainerOwner,
		ContainerSpec,
		WorkerSpec,
		ContainerPlacementStrategy,
		db.ContainerMetadata,
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
		runtime.ProcessSpec,
		runtime.StartingEventDelegate,
		db.UsedResourceCache,
		resource.Resource,
	) (GetResult, error)
}

func NewClient(pool Pool,
	provider WorkerProvider,
	compression compression.Compression,
	workerPollingInterval time.Duration,
	workerStatusPublishInterval time.Duration,
	enabledP2pStreaming bool,
	p2pStreamingTimeout time.Duration,
	resourceCacheFactory db.ResourceCacheFactory,
) *client {
	return &client{
		pool:                        pool,
		provider:                    provider,
		compression:                 compression,
		workerPollingInterval:       workerPollingInterval,
		workerStatusPublishInterval: workerStatusPublishInterval,
		enabledP2pStreaming:         enabledP2pStreaming,
		p2pStreamingTimeout:         p2pStreamingTimeout,
		resourceCacheFactory:        resourceCacheFactory,
	}
}

type client struct {
	pool                        Pool
	provider                    WorkerProvider
	compression                 compression.Compression
	workerPollingInterval       time.Duration
	workerStatusPublishInterval time.Duration
	enabledP2pStreaming         bool
	p2pStreamingTimeout         time.Duration
	resourceCacheFactory        db.ResourceCacheFactory
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
	FromCache     bool
}

type processStatus struct {
	processStatus int
	processErr    error
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

func (client *client) RunCheckStep(
	ctx context.Context,
	logger lager.Logger,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	workerSpec WorkerSpec,
	strategy ContainerPlacementStrategy,
	containerMetadata db.ContainerMetadata,
	processSpec runtime.ProcessSpec,
	eventDelegate runtime.StartingEventDelegate,
	checkable resource.Resource,
) (CheckResult, error) {
	if containerSpec.ImageSpec.ImageArtifact != nil {
		err := client.wireImageVolume(logger, &containerSpec.ImageSpec)
		if err != nil {
			return CheckResult{}, err
		}
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
		return CheckResult{}, fmt.Errorf("find or choose worker for container: %w", err)
	}

	eventDelegate.SelectedWorker(logger, chosenWorker.Name())

	container, err := chosenWorker.FindOrCreateContainer(
		ctx,
		logger,
		owner,
		containerMetadata,
		containerSpec,
	)
	if err != nil {
		return CheckResult{}, err
	}

	eventDelegate.Starting(logger)

	versions, err := checkable.Check(ctx, processSpec, container)
	if err != nil {
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

	eventDelegate.SelectedWorker(logger, chosenWorker.Name())

	if strategy.ModifiesActiveTasks() {
		defer decreaseActiveTasks(logger.Session("decrease-active-tasks"), chosenWorker)
	}

	container, err := chosenWorker.FindOrCreateContainer(
		ctx,
		logger,
		owner,
		metadata,
		containerSpec,
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
	processSpec runtime.ProcessSpec,
	eventDelegate runtime.StartingEventDelegate,
	resourceCache db.UsedResourceCache,
	resource resource.Resource,
) (GetResult, error) {
	if containerSpec.ImageSpec.ImageArtifact != nil {
		err := client.wireImageVolume(logger, &containerSpec.ImageSpec)
		if err != nil {
			return GetResult{}, err
		}
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
		return GetResult{}, err
	}

	eventDelegate.SelectedWorker(logger, chosenWorker.Name())

	sign, err := resource.Signature()
	if err != nil {
		return GetResult{}, err
	}

	destVolume, ok, err := client.findAndStreamInResource(ctx, logger, chosenWorker, containerSpec.TeamID, resourceCache.ID())
	if err == nil && ok {
		logger.Info("EVAN:GetStep got volume directly", lager.Data{"team": containerSpec.TeamID, "rcId": resourceCache.ID()})
		metadata, err := resourceCache.LoadVersionMetadata()
		if err != nil {
			logger.Error("EVAN:failed to load version metadata", err)
			return GetResult{}, err
		}
		return GetResult{
			ExitStatus: 0,
			VersionResult: runtime.VersionResult{
				Version:  resourceCache.Version(),
				Metadata: metadata,
			},
			GetArtifact: runtime.GetArtifact{destVolume.Handle()},
			FromCache:   true,
		}, nil
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
		resourceCache,
		lockName,
	)
	return getResult, err
}

func (client *client) findAndStreamInResource(ctx context.Context, logger lager.Logger, destWorker Worker, teamId int, rcId int) (Volume, bool, error) {
	logger.Info("EVAN:enter findAndStreamInResource====================",
		lager.Data{"destWorker": destWorker.Name(), "teamId": teamId, "rcId": rcId})

	workers, err := client.pool.FindWorkersForResourceCache(logger, teamId, rcId)
	if err != nil {
		logger.Error("EVAN:findAndStreamInResource: failed to find worker resource cache", err)
		return nil, false, err
	}
	if len(workers) == 0 {
		logger.Info("EVAN:findAndStreamInResource: not find worker resource cache")
		return nil, false, nil
	}

	resourceCache, _, err := client.resourceCacheFactory.FindResourceCacheByID(rcId)

	for _, w := range workers {
		if w.Name() == destWorker.Name() {
			volume, found, err := destWorker.FindVolumeForResourceCache(logger, resourceCache)
			if err != nil {
				logger.Error("EVAN:findAndStreamInResource: volume on local, but error", err, lager.Data{"worker": destWorker.Name(), "rcId": rcId})
				return nil, false, nil
			}
			if !found {
				logger.Error("EVAN:findAndStreamInResource: volume on local, but not found", err, lager.Data{"worker": destWorker.Name(), "rcId": rcId})
				return nil, false, nil
			}

			logger.Info("EVAN:findAndStreamInResource: find volume on destWorker", lager.Data{"team": teamId, "rcId": resourceCache.ID(), "destWorker": destWorker.Name()})
			return volume, true, nil
		}
	}

	for _, sourceWorker := range workers {
		logger.Debug("EVAN:findAndStreamInResource:try source worker", lager.Data{"worker": sourceWorker.Name()})
		volume, found, err := sourceWorker.FindVolumeForResourceCache(logger, resourceCache)
		if err != nil {
			logger.Info("EVAN:failed-to-find-resource-cache", lager.Data{"worker": sourceWorker.Name(), "rcId": rcId})
			continue
		}
		if !found {
			logger.Info("EVAN:not-find-resource-cache", lager.Data{"worker": sourceWorker.Name(), "rcId": rcId})
			continue
		}

		source := NewStreamableArtifactSource(
			&runtime.GetArtifact{
				VolumeHandle: volume.Handle(),
			},
			volume,
			client.compression,
			client.enabledP2pStreaming,
			client.p2pStreamingTimeout,
			client.resourceCacheFactory,
		)

		destVolume, err := destWorker.CreateVolume(
			logger,
			VolumeSpec{Strategy: baggageclaim.EmptyStrategy{}},
			teamId,
			db.VolumeTypeResource,
		)
		if err != nil {
			logger.Error("EVAN:failed to create dest volume", err)
			return nil, false, err
		}

		err = source.StreamTo(ctx, destVolume)
		if err != nil {
			logger.Error("EVAN:failed to stream to", err)
			return nil, false, err
		}

		logger.Info("EVAN:streamed get resource", lager.Data{"handle": destVolume.Handle()})
		return destVolume, true, nil
	}

	return nil, false, nil
}

func (client *client) RunPutStep(
	ctx context.Context,
	logger lager.Logger,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	workerSpec WorkerSpec,
	strategy ContainerPlacementStrategy,
	metadata db.ContainerMetadata,
	spec runtime.ProcessSpec,
	eventDelegate runtime.StartingEventDelegate,
	resource resource.Resource,
) (PutResult, error) {
	if containerSpec.ImageSpec.ImageArtifact != nil {
		err := client.wireImageVolume(logger, &containerSpec.ImageSpec)
		if err != nil {
			return PutResult{}, err
		}
	}

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

	eventDelegate.SelectedWorker(logger, chosenWorker.Name())

	container, err := chosenWorker.FindOrCreateContainer(
		ctx,
		logger,
		owner,
		metadata,
		containerSpec,
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
	return source.StreamFile(ctx, filePath)
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
		chosenWorker    Worker
		activeTasksLock lock.Lock
		lockAcquired    bool
		elapsed         time.Duration
		err             error
	)

	started := time.Now()
	workerPollingTicker := time.NewTicker(client.workerPollingInterval)
	defer workerPollingTicker.Stop()
	workerStatusPublishTicker := time.NewTicker(client.workerStatusPublishInterval)
	defer workerStatusPublishTicker.Stop()

	tasksWaitingLabels := metric.TasksWaitingLabels{
		TeamId:     strconv.Itoa(workerSpec.TeamID),
		WorkerTags: strings.Join(workerSpec.Tags, "_"),
		Platform:   workerSpec.Platform,
	}

	for {
		if strategy.ModifiesActiveTasks() {
			if activeTasksLock, lockAcquired, err = lockFactory.Acquire(logger, lock.NewActiveTasksLockID()); err != nil {
				return nil, err
			}

			if !lockAcquired {
				time.Sleep(time.Second)
				continue
			}
		}

		if chosenWorker, err = client.pool.FindOrChooseWorkerForContainer(
			ctx,
			logger,
			owner,
			containerSpec,
			workerSpec,
			strategy,
		); err != nil {
			if chosenWorker == nil && !strategy.ModifiesActiveTasks() {
				// only the limit-active-tasks placement strategy waits for a
				// worker to become available. All others should error out for
				// now
				return nil, err
			}
		}

		if !strategy.ModifiesActiveTasks() {
			return chosenWorker, nil
		}

		select {
		case <-ctx.Done():
			logger.Info("aborted-waiting-worker")
			e := multierror.Append(err, activeTasksLock.Release(), ctx.Err())
			return nil, e
		default:
		}

		if chosenWorker != nil {
			err = increaseActiveTasks(logger,
				client.pool,
				chosenWorker,
				activeTasksLock,
				owner,
				containerSpec,
				workerSpec)

			if elapsed > 0 {
				message := fmt.Sprintf("Found a free worker after waiting %s.\n", elapsed.Round(1*time.Second))
				writeOutputMessage(logger, outputWriter, message)
				metric.TasksWaitingDuration{
					Labels:   tasksWaitingLabels,
					Duration: elapsed,
				}.Emit(logger)
			}

			return chosenWorker, err
		}

		err := activeTasksLock.Release()
		if err != nil {
			return nil, err
		}

		// Increase task waiting only once
		if elapsed == 0 {
			_, ok := metric.Metrics.TasksWaiting[tasksWaitingLabels]
			if !ok {
				metric.Metrics.TasksWaiting[tasksWaitingLabels] = &metric.Gauge{}
			}
			metric.Metrics.TasksWaiting[tasksWaitingLabels].Inc()
			defer metric.Metrics.TasksWaiting[tasksWaitingLabels].Dec()
		}

		elapsed = waitForWorker(logger,
			workerPollingTicker,
			workerStatusPublishTicker,
			outputWriter,
			started)
	}
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

			source := NewStreamableArtifactSource(artifact, artifactVolume, client.compression, client.enabledP2pStreaming, client.p2pStreamingTimeout, client.resourceCacheFactory)
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

	spec.ImageArtifactSource = NewStreamableArtifactSource(imageArtifact, artifactVolume, client.compression, client.enabledP2pStreaming, client.p2pStreamingTimeout, client.resourceCacheFactory)

	return nil
}

func decreaseActiveTasks(logger lager.Logger, w Worker) {
	err := w.DecreaseActiveTasks()
	if err != nil {
		logger.Error("failed-to-decrease-active-tasks", err)
		return
	}
}

func lockName(resourceJSON []byte, workerName string) string {
	jsonRes := append(resourceJSON, []byte(workerName)...)
	return fmt.Sprintf("%x", sha256.Sum256(jsonRes))
}

func waitForWorker(
	logger lager.Logger,
	waitForWorkerTicker, workerStatusTicker *time.Ticker,
	outputWriter io.Writer,
	started time.Time) (elapsed time.Duration) {

	select {
	case <-waitForWorkerTicker.C:
		elapsed = time.Since(started)

	case <-workerStatusTicker.C:
		message := "All workers are busy at the moment, please stand-by.\n"
		writeOutputMessage(logger, outputWriter, message)
		elapsed = time.Since(started)
	}

	return elapsed
}

func writeOutputMessage(logger lager.Logger, outputWriter io.Writer, message string) {
	_, err := outputWriter.Write([]byte(message))
	if err != nil {
		logger.Error("failed-to-report-status", err)
	}
}

func increaseActiveTasks(
	logger lager.Logger,
	pool Pool,
	chosenWorker Worker,
	activeTasksLock lock.Lock,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	workerSpec WorkerSpec) (err error) {

	var existingContainer bool
	defer release(activeTasksLock, err)

	existingContainer, err = pool.ContainerInWorker(logger, owner, workerSpec)
	if err != nil {
		return err
	}

	if !existingContainer {
		if err = chosenWorker.IncreaseActiveTasks(); err != nil {
			logger.Error("failed-to-increase-active-tasks", err)
		}
	}

	return err
}

func release(activeTasksLock lock.Lock, err error) {
	releaseErr := activeTasksLock.Release()
	if releaseErr != nil {
		err = multierror.Append(err, releaseErr)
	}
}
