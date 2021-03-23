package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/builds"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/worker"
	"github.com/hashicorp/go-multierror"
)

func NewTaskDelegate(
	build db.Build,
	planID atc.PlanID,
	state exec.RunState,
	clock clock.Clock,
	policyChecker policy.Checker,
	globalSecrets creds.Secrets,
	artifactSourcer worker.ArtifactSourcer,
	dbWorkerFactory db.WorkerFactory,
	lockFactory lock.LockFactory,
) exec.TaskDelegate {
	return &taskDelegate{
		BuildStepDelegate: NewBuildStepDelegate(build, planID, state, clock, policyChecker, globalSecrets, artifactSourcer),

		eventOrigin: event.Origin{ID: event.OriginID(planID)},
		planID:      planID,
		build:       build,
		clock:       clock,

		dbWorkerFactory: dbWorkerFactory,
		lockFactory:     lockFactory,
	}
}

type taskDelegate struct {
	exec.BuildStepDelegate

	planID      atc.PlanID
	config      atc.TaskConfig
	build       db.Build
	eventOrigin event.Origin
	clock       clock.Clock

	dbWorkerFactory db.WorkerFactory
	lockFactory     lock.LockFactory
}

func (d *taskDelegate) SetTaskConfig(config atc.TaskConfig) {
	d.config = config
}

func (d *taskDelegate) SelectWorker(
	ctx context.Context,
	pool worker.Pool,
	owner db.ContainerOwner,
	containerSpec worker.ContainerSpec,
	workerSpec worker.WorkerSpec,
	strategy worker.ContainerPlacementStrategy,
	workerPollingInterval, workerStatusPublishInterval time.Duration,
) (worker.Client, error) {
	logger := lagerctx.FromContext(ctx)

	started := time.Now()
	workerPollingTicker := time.NewTicker(workerPollingInterval)
	defer workerPollingTicker.Stop()
	workerStatusPublishTicker := time.NewTicker(workerStatusPublishInterval)
	defer workerStatusPublishTicker.Stop()

	tasksWaitingLabels := metric.TasksWaitingLabels{
		TeamId:     strconv.Itoa(workerSpec.TeamID),
		WorkerTags: strings.Join(workerSpec.Tags, "_"),
		Platform:   workerSpec.Platform,
	}

	trySelectWorker := func() (worker.Client, error) {
		var (
			activeTasksLock lock.Lock
			lockAcquired    bool
			err             error
		)
		if strategy.ModifiesActiveTasks() {
			for {
				activeTasksLock, lockAcquired, err = d.lockFactory.Acquire(logger, lock.NewActiveTasksLockID())
				if err != nil {
					return nil, err
				}

				if lockAcquired {
					defer activeTasksLock.Release()
					break
				}
				// retry after a delay
				time.Sleep(time.Second)
			}
		}

		chosenWorker, err := pool.SelectWorker(
			ctx,
			owner,
			containerSpec,
			workerSpec,
			strategy,
		)
		if err != nil {
			// only the limit-active-tasks placement strategy waits for a
			// worker to become available. All others should error out for now
			allWorkersFullError := worker.NoWorkerFitContainerPlacementStrategyError{Strategy: "limit-active-tasks"}
			if !errors.Is(err, allWorkersFullError) {
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

		if chosenWorker == nil {
			return nil, nil
		}

		err = d.increaseActiveTasks(
			logger,
			activeTasksLock,
			pool,
			chosenWorker,
			owner,
			workerSpec,
		)
		if err != nil {
			logger.Error("failed-to-increase-active-tasks", err)
		}

		return chosenWorker, err
	}

	var elapsed time.Duration
	for {
		chosenWorker, err := trySelectWorker()
		if err != nil {
			return nil, err
		}
		if chosenWorker != nil {
			if elapsed > 0 {
				message := fmt.Sprintf("Found a free worker after waiting %s.\n", elapsed.Round(1*time.Second))
				d.writeOutputMessage(logger, message)
				metric.TasksWaitingDuration{
					Labels:   tasksWaitingLabels,
					Duration: elapsed,
				}.Emit(logger)
			}
			return chosenWorker, nil
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

		elapsed = d.waitForWorker(
			logger,
			workerPollingTicker,
			workerStatusPublishTicker,
			started,
		)
	}
}

func (d taskDelegate) increaseActiveTasks(
	logger lager.Logger,
	activeTasksLock lock.Lock,
	pool worker.Pool,
	chosenWorker worker.Client,
	owner db.ContainerOwner,
	workerSpec worker.WorkerSpec,
) error {
	var existingContainer bool
	existingContainer, err := pool.ContainerInWorker(logger, owner, workerSpec)
	if err != nil {
		return err
	}

	if existingContainer {
		return nil
	}

	dbWorker, err := d.findDBWorker(chosenWorker.Name())
	if err != nil {
		return err
	}

	return dbWorker.IncreaseActiveTasks()
}

func (d taskDelegate) decreaseActiveTasks(chosenWorker worker.Client) error {
	dbWorker, err := d.findDBWorker(chosenWorker.Name())
	if err != nil {
		return err
	}
	return dbWorker.DecreaseActiveTasks()
}

func (d taskDelegate) findDBWorker(name string) (db.Worker, error) {
	dbWorker, found, err := d.dbWorkerFactory.GetWorker(name)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("chosen worker %s is no longer available", name)
	}
	return dbWorker, nil
}

func (d taskDelegate) writeOutputMessage(logger lager.Logger, message string) {
	_, err := d.BuildStepDelegate.Stdout().Write([]byte(message))
	if err != nil {
		logger.Error("failed-to-report-status", err)
	}
}

func (d taskDelegate) waitForWorker(
	logger lager.Logger,
	waitForWorkerTicker, workerStatusTicker *time.Ticker,
	started time.Time,
) (elapsed time.Duration) {
	select {
	case <-waitForWorkerTicker.C:
		elapsed = time.Since(started)

	case <-workerStatusTicker.C:
		d.writeOutputMessage(logger, "All workers are busy at the moment, please stand-by.\n")
		elapsed = time.Since(started)
	}

	return elapsed
}

func (d *taskDelegate) Initializing(logger lager.Logger) {
	err := d.build.SaveEvent(event.InitializeTask{
		Origin:     d.eventOrigin,
		Time:       d.clock.Now().Unix(),
		TaskConfig: event.ShadowTaskConfig(d.config),
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-task-event", err)
		return
	}

	logger.Info("initializing")
}

func (d *taskDelegate) Starting(logger lager.Logger) {
	err := d.build.SaveEvent(event.StartTask{
		Origin:     d.eventOrigin,
		Time:       d.clock.Now().Unix(),
		TaskConfig: event.ShadowTaskConfig(d.config),
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-task-event", err)
		return
	}

	logger.Debug("starting")
}

func (d *taskDelegate) Finished(
	logger lager.Logger,
	exitStatus exec.ExitStatus,
	strategy worker.ContainerPlacementStrategy,
	chosenWorker worker.Client,
) {
	// PR#4398: close to flush stdout and stderr
	d.Stdout().(io.Closer).Close()
	d.Stderr().(io.Closer).Close()

	err := d.build.SaveEvent(event.FinishTask{
		ExitStatus: int(exitStatus),
		Time:       d.clock.Now().Unix(),
		Origin:     d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-finish-event", err)
		return
	}

	if strategy.ModifiesActiveTasks() {
		if err := d.decreaseActiveTasks(chosenWorker); err != nil {
			logger.Error("failed-to-decrease-active-tasks", err)
		}
	}

	logger.Info("finished", lager.Data{"exit-status": exitStatus})
}

func (d *taskDelegate) FetchImage(
	ctx context.Context,
	image atc.ImageResource,
	types atc.VersionedResourceTypes,
	privileged bool,
	stepTags atc.Tags,
) (worker.ImageSpec, error) {
	image.Name = "image"

	checkPlan, getPlan, _ := builds.FetchImagePlan(d.planID, image, types, stepTags)

	if checkPlan != nil {
		err := d.build.SaveEvent(event.ImageCheck{
			Time: d.clock.Now().Unix(),
			Origin: event.Origin{
				ID: event.OriginID(d.planID),
			},
			PublicPlan: checkPlan.Public(),
		})
		if err != nil {
			return worker.ImageSpec{}, err
		}
	}

	err := d.build.SaveEvent(event.ImageGet{
		Time: d.clock.Now().Unix(),
		Origin: event.Origin{
			ID: event.OriginID(d.planID),
		},
		PublicPlan: getPlan.Public(),
	})
	if err != nil {
		return worker.ImageSpec{}, err
	}

	return d.BuildStepDelegate.FetchImage(ctx, getPlan, checkPlan, privileged)
}
