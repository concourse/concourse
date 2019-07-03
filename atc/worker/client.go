package worker

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"context"
	"fmt"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
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
		chan string,
	) (int, []VolumeMount, error)
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
	events chan string,
) (int, []VolumeMount, error) {
	chosenWorker, err := client.pool.FindOrChooseWorkerForContainer(
		ctx,
		logger,
		owner,
		containerSpec,
		workerSpec,
		strategy,
	)
	if err != nil {
		return -1, []VolumeMount{}, err
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
		return -1, []VolumeMount{}, err
	}

	// container already exited
	exitStatusProp, err := container.Property(taskExitStatusPropertyName)
	if err == nil {
		logger.Info("already-exited", lager.Data{"status": exitStatusProp})

		status, err := strconv.Atoi(exitStatusProp)
		if err != nil {
			return -1, []VolumeMount{}, err
		}

		return status, container.VolumeMounts(), nil
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

		events <- "Starting"

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
			return -1, []VolumeMount{}, err
		}
	}

	logger.Info("attached")

	exited := make(chan struct{})
	var processStatus int
	var processErr error

	go func() {
		processStatus, processErr = process.Wait()
		close(exited)
	}()

	select {
	case <-ctx.Done():
		err = container.Stop(false)
		if err != nil {
			logger.Error("stopping-container", err)
		}

		<-exited

		return -1, container.VolumeMounts(), ctx.Err()

	case <-exited:
		if processErr != nil {
			return processStatus, []VolumeMount{}, processErr
		}

		err = container.SetProperty(taskExitStatusPropertyName, fmt.Sprintf("%d", processStatus))
		if err != nil {
			return processStatus, []VolumeMount{}, err
		}

		return processStatus, container.VolumeMounts(), nil
	}
}
