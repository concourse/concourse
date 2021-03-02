package engine

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/worker"
)

func NewTaskDelegate(
	build db.Build,
	planID atc.PlanID,
	state exec.RunState,
	clock clock.Clock,
	policyChecker policy.Checker,
	artifactSourcer worker.ArtifactSourcer,
	dbWorkerFactory db.WorkerFactory,
	lockFactory lock.LockFactory,
) exec.TaskDelegate {
	return &taskDelegate{
		BuildStepDelegate: NewBuildStepDelegate(build, planID, state, clock, policyChecker, artifactSourcer),

		eventOrigin: event.Origin{ID: event.OriginID(planID)},
		build:       build,
		clock:       clock,

		dbWorkerFactory: dbWorkerFactory,
		lockFactory:     lockFactory,
	}
}

type taskDelegate struct {
	exec.BuildStepDelegate

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

	var elapsed time.Duration
	for {
		chosenWorker, err := pool.SelectWorker(
			ctx,
			owner,
			containerSpec,
			workerSpec,
			strategy,
		)

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

	logger.Info("finished", lager.Data{"exit-status": exitStatus})
}
