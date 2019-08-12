package worker

import (
	"context"
	"fmt"
	"io"
	"path"
	"strconv"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/runtime"
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
	lockFactory lock.LockFactory,
	owner db.ContainerOwner,
	containerSpec ContainerSpec,
	workerSpec WorkerSpec,
	strategy ContainerPlacementStrategy,
	metadata db.ContainerMetadata,
	imageSpec ImageFetcherSpec,
	processSpec TaskProcessSpec,
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
		defer decreaseActiveTasks(logger.Session("decrease-active-tasks"), lockFactory, chosenWorker)
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

func decreaseActiveTasks(logger lager.Logger, lockFactory lock.LockFactory, w Worker) {
	var (
		activeTasksLock lock.Lock
		err             error
		acquired        bool
	)
	for {
		activeTasksLock, acquired, err = lockFactory.Acquire(logger, lock.NewActiveTasksLockID())
		if err != nil {
			logger.Error("failed-to-acquire-active-tasks-lock", err)
			return
		}

		if !acquired {
			time.Sleep(time.Second)
			continue
		} else {
			break
		}
	}

	err = w.DecreaseActiveTasks()
	if err != nil {
		logger.Error("failed-to-decrease-active-tasks", err)
		return
	}

	err = activeTasksLock.Release()
	if err != nil {
		logger.Error("failed-to-release-active-tasks-lock", err)
		return
	}

	return
}

type processStatus struct {
	processStatus int
	processErr    error
}
