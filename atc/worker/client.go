package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
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
	RunTaskStep(
		context.Context,
		lager.Logger,
		lock.LockFactory,
		db.ContainerOwner,
		ContainerSpec,
		WorkerSpec,
		ContainerPlacementStrategy,
		db.ContainerMetadata,
		ImageFetcherSpec,
		ProcessSpec,
		chan runtime.Event,
	) TaskResult
	RunPutStep(
		context.Context,
		lager.Logger,
		db.ContainerOwner,
		ContainerSpec,
		WorkerSpec,
		atc.Source,
		atc.Params,
		ContainerPlacementStrategy,
		db.ContainerMetadata,
		ImageFetcherSpec,
		string,
		ProcessSpec,
		chan runtime.Event,
	) PutResult
	RunGetStep(
		context.Context,
		lager.Logger,
		db.ContainerOwner,
		ContainerSpec,
		WorkerSpec,
		ContainerPlacementStrategy,
		db.ContainerMetadata,
		atc.VersionedResourceTypes,
		atc.Source,
		atc.Params,
		string,
		string,
		Fetcher,
		ImageFetchingDelegate,
		db.UsedResourceCache,
		ProcessSpec,
		chan runtime.Event,
	) (GetResult, error)
	StreamFileFromArtifact(ctx context.Context, logger lager.Logger, artifact runtime.Artifact, filePath string) (io.ReadCloser, error)
}

func NewClient(pool Pool, provider WorkerProvider) *client {
	return &client{
		pool:     pool,
		provider: provider,
	}
}

type client struct {
	pool     Pool
	provider WorkerProvider
}

type TaskResult struct {
	Status       int
	VolumeMounts []VolumeMount
	Err          error
}

type PutResult struct {
	Status        int
	VersionResult runtime.VersionResult
	Err           error
}

type GetResult struct {
	Status        int
	VersionResult runtime.VersionResult
	GetArtifact   runtime.GetArtifact
	Err           error
}

type ProcessSpec struct {
	Path         string
	Args         []string
	Dir          string
	User         string
	StdoutWriter io.Writer
	StderrWriter io.Writer
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

func (client *client) RunTaskStep(
	ctx context.Context,
	logger lager.Logger,
	lockFactory lock.LockFactory,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	workerSpec WorkerSpec,
	strategy ContainerPlacementStrategy,
	metadata db.ContainerMetadata,
	imageFetcherSpec ImageFetcherSpec,
	processSpec ProcessSpec,
	events chan runtime.Event,
) TaskResult {
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
		return TaskResult{Status: -1, VolumeMounts: []VolumeMount{}, Err: err}
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
		return TaskResult{Status: -1, VolumeMounts: []VolumeMount{}, Err: err}
	}

	// container already exited
	exitStatusProp, _ := container.Properties()
	code := exitStatusProp[taskExitStatusPropertyName]
	if code != "" {
		logger.Info("already-exited", lager.Data{"status": taskExitStatusPropertyName})

		status, err := strconv.Atoi(code)
		if err != nil {
			return TaskResult{-1, []VolumeMount{}, err}
		}

		return TaskResult{Status: status, VolumeMounts: container.VolumeMounts(), Err: nil}

	}

	processIO := garden.ProcessIO{
		Stdout: processSpec.StdoutWriter,
		Stderr: processSpec.StderrWriter,
	}

	process, err := container.Attach(context.Background(), taskProcessID, processIO)
	if err == nil {
		logger.Info("already-running")
	} else {
		logger.Info("spawning")

		events <- runtime.Event{
			EventType: runtime.StartingEvent,
		}

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
			return TaskResult{Status: -1, VolumeMounts: []VolumeMount{}, Err: err}
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
		return TaskResult{Status: status.processStatus, VolumeMounts: container.VolumeMounts(), Err: ctx.Err()}

	case status := <-exitStatusChan:
		if status.processErr != nil {
			return TaskResult{Status: status.processStatus, VolumeMounts: []VolumeMount{}, Err: status.processErr}
		}

		err = container.SetProperty(taskExitStatusPropertyName, fmt.Sprintf("%d", status.processStatus))
		if err != nil {
			return TaskResult{Status: status.processStatus, VolumeMounts: []VolumeMount{}, Err: err}
		}
		return TaskResult{Status: status.processStatus, VolumeMounts: container.VolumeMounts(), Err: nil}
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
	resourceTypes atc.VersionedResourceTypes,
	source atc.Source,
	params atc.Params,
	resourceDir string,
	resourceInstanceSignature string,
	resourceFetcher Fetcher,
	delegate ImageFetchingDelegate,
	cache db.UsedResourceCache,
	processSpec ProcessSpec,
	events chan runtime.Event,
) (GetResult, error) {
	vr := runtime.VersionResult{}
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

	events <- runtime.Event{
		EventType: runtime.StartingEvent,
	}

	// start of dependency on resource -> worker
	getResult, err := resourceFetcher.Fetch(
		ctx,
		logger,
		containerMetadata,
		chosenWorker,
		containerSpec,
		processSpec,
		resourceTypes,
		source,
		params,
		owner,
		resourceDir,
		resourceInstanceSignature,
		delegate,
		cache,
	)
	if err != nil {
		logger.Error("failed-to-fetch-resource", err)

		// TODO Define an error on Event for Concourse system errors or define an Concourse system error Exit Status
		events <- runtime.Event{
			EventType:     runtime.FinishedEvent,
			ExitStatus:    500,
			VersionResult: vr,
		}
		return GetResult{}, err
	}

	events <- runtime.Event{
		EventType:     runtime.FinishedEvent,
		ExitStatus:    getResult.Status,
		VersionResult: getResult.VersionResult,
	}
	return getResult, nil
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
	source atc.Source,
	params atc.Params,
	strategy ContainerPlacementStrategy,
	metadata db.ContainerMetadata,
	imageFetcherSpec ImageFetcherSpec,
	resourceDir string,
	spec ProcessSpec,
	events chan runtime.Event,
) PutResult {

	vr := runtime.VersionResult{}

	chosenWorker, err := client.pool.FindOrChooseWorkerForContainer(
		ctx,
		logger,
		owner,
		containerSpec,
		workerSpec,
		strategy,
	)
	if err != nil {
		return PutResult{Status: -1, VersionResult: vr, Err: err}
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
		return PutResult{Status: -1, VersionResult: vr, Err: err}
	}

	// container already exited
	exitStatusProp, err := container.Property(taskExitStatusPropertyName)
	if err == nil {
		logger.Info("already-exited", lager.Data{"status": exitStatusProp})
		return PutResult{Status: -1, VersionResult: vr, Err: nil}
	}

	var result PutResult
	err = RunScript(
		ctx,
		container,
		spec.Path,
		spec.Args,
		runtime.PutRequest{
			Params: params,
			Source: source,
		},
		&vr,
		spec.StderrWriter,
		true,
		events,
	)

	if err != nil {
		if failErr, ok := err.(ErrResourceScriptFailed); ok {
			result = PutResult{failErr.ExitStatus, runtime.VersionResult{}, failErr}
		} else {
			result = PutResult{-1, runtime.VersionResult{}, err}
		}
	} else {
		result = PutResult{0, vr, nil}
	}
	return result
}

func (client *client) StreamFileFromArtifact(ctx context.Context, logger lager.Logger, artifact runtime.Artifact, filePath string) (io.ReadCloser, error) {
	var getArtifact runtime.GetArtifact
	var ok bool

	if getArtifact, ok = artifact.(runtime.GetArtifact); !ok {
		return nil, errors.New("unrecognized task config artifact type")
	}

	artifactVolume, found, err := client.FindVolume(logger, 0, getArtifact.ID())
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, baggageclaim.ErrVolumeNotFound
	}

	source := getArtifactSource{
		artifact: getArtifact,
		volume:   artifactVolume,
	}
	return source.StreamFile(ctx, logger, filePath)
}
