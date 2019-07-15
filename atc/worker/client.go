package worker

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"context"
	"fmt"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
	"io"
	"path"
	"strconv"
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
		db.ContainerOwner,
		ContainerSpec,
		WorkerSpec,
		ContainerPlacementStrategy,
		db.ContainerMetadata,
		ImageFetcherSpec,
		TaskProcessSpec,
		chan runtime.Event,
	) TaskResult
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

type TaskProcessSpec struct {
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
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	workerSpec WorkerSpec,
	strategy ContainerPlacementStrategy,
	metadata db.ContainerMetadata,
	imageSpec ImageFetcherSpec,
	processSpec TaskProcessSpec,
	events chan runtime.Event,
) TaskResult {
	chosenWorker, err := client.pool.FindOrChooseWorkerForContainer(
		ctx,
		logger,
		owner,
		containerSpec,
		workerSpec,
		strategy,
	)
	if err != nil {
		return TaskResult{- 1, []VolumeMount{}, err}
	}

	container, err := chosenWorker.FindOrCreateContainer(
		ctx,
		logger,
		imageSpec.Delegate,
		owner,
		metadata,
		containerSpec,
		imageSpec.ResourceTypes,
	)

	if err != nil {
		return TaskResult{Status:- 1, VolumeMounts: []VolumeMount{}, Err: err}
	}

	// container already exited
	exitStatusProp, err := container.Property(taskExitStatusPropertyName)
	if err == nil {
		logger.Info("already-exited", lager.Data{"status": exitStatusProp})

		status, err := strconv.Atoi(exitStatusProp)
		if err != nil {
			return TaskResult{- 1, []VolumeMount{}, err}
		}

		return TaskResult { Status: status, VolumeMounts: container.VolumeMounts(), Err: nil }
	}

	processIO := garden.ProcessIO{
		Stdout: processSpec.StdoutWriter,
		Stderr: processSpec.StderrWriter,
	}

	process, err := container.Attach(taskProcessID, processIO)
	if err == nil {
		logger.Info("already-running")
	} else {
		logger.Info("spawning")

		events <- runtime.Event{
			EventType: runtime.StartingEvent,
		}

		process, err = container.Run(
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

	exited := make(chan string)

	status := &processStatus{}
	go func(status *processStatus) {
		status.processStatus, status.processErr = process.Wait()
		exited <- "done"
	}(status)

	select {
	case <-ctx.Done():
		err = container.Stop(false)
		if err != nil {
			logger.Error("stopping-container", err)
		}

		<-exited

		return TaskResult{ Status: status.processStatus, VolumeMounts: container.VolumeMounts(), Err: ctx.Err() }

	case <-exited:
		if status.processErr != nil {
			return TaskResult{ Status: status.processStatus, VolumeMounts: []VolumeMount{}, Err: status.processErr }
		}

		err = container.SetProperty(taskExitStatusPropertyName, fmt.Sprintf("%d", status.processStatus))
		if err != nil {
			return TaskResult { Status: status.processStatus, VolumeMounts: []VolumeMount{}, Err: err }
		}

		return TaskResult{ Status: status.processStatus, VolumeMounts: container.VolumeMounts(), Err: nil }
	}
}

type processStatus struct {
	processStatus int
	processErr error
}