package scheduler

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/metric"
)

//go:generate counterfeiter . BuildScheduler

type BuildScheduler interface {
	Schedule(
		logger lager.Logger,
		versions *db.VersionsDB,
		job db.Job,
		resources db.Resources,
		resourceTypes atc.VersionedResourceTypes,
	) error
}

var errPipelineRemoved = errors.New("pipeline removed")

const maxJobsInFlight = 32

var guardJobScheduling = make(chan struct{}, maxJobsInFlight)

type Runner struct {
	Logger    lager.Logger
	Pipeline  db.Pipeline
	Scheduler BuildScheduler
	Noop      bool
	Interval  time.Duration
}

func (runner *Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	if runner.Interval == 0 {
		panic("unconfigured scheduler interval")
	}

	runner.Logger.Info("start", lager.Data{
		"interval": runner.Interval.String(),
	})

	defer runner.Logger.Info("done")

dance:
	for {
		err := runner.tick(runner.Logger.Session("tick"))
		if err != nil {
			return err
		}

		select {
		case <-time.After(runner.Interval):
		case <-signals:
			break dance
		}
	}

	return nil
}

func (runner *Runner) tick(logger lager.Logger) error {
	if runner.Noop {
		return nil
	}

	start := time.Now()

	defer func() {
		metric.SchedulingFullDuration{
			PipelineName: runner.Pipeline.Name(),
			Duration:     time.Since(start),
		}.Emit(logger)
	}()

	found, err := runner.Pipeline.Reload()
	if err != nil {
		logger.Error("failed-to-update-pipeline-config", err)
		return nil
	}

	if !found {
		return errPipelineRemoved
	}

	versions, err := runner.Pipeline.LoadVersionsDB()
	if err != nil {
		logger.Error("failed-to-load-versions-db", err)
		return err
	}

	jobs, err := runner.Pipeline.Jobs()
	if err != nil {
		logger.Error("failed-to-get-jobs", err)
		return err
	}

	for _, job := range jobs {
		schedulingLock, acquired, err := job.AcquireSchedulingLock(logger, runner.Interval)
		if err != nil {
			logger.Error("failed-to-acquire-scheduling-lock", err)
			return nil
		}

		if !acquired {
			continue
		}

		guardJobScheduling <- struct{}{} // would block if guard channel is already filled
		go func(j db.Job) {
			runner.scheduleJob(logger, schedulingLock, versions, j)
			<-guardJobScheduling
		}(job)
	}

	return err
}

func (runner *Runner) scheduleJob(logger lager.Logger, schedulingLock lock.Lock, versions *db.VersionsDB, job db.Job) {
	defer schedulingLock.Release()

	start := time.Now()

	metric.SchedulingLoadVersionsDuration{
		PipelineName: runner.Pipeline.Name(),
		Duration:     time.Since(start),
	}.Emit(logger)

	found, err := job.Reload()
	if err != nil {
		logger.Error("failed-to-update-job-config", err)
		return
	}

	if !found {
		logger.Error("job-not-found", err)
		return
	}

	resources, err := runner.Pipeline.Resources()
	if err != nil {
		logger.Error("failed-to-get-resources", err)
		return
	}

	sLog := logger.Session("scheduling")
	jStart := time.Now()

	err = runner.Scheduler.Schedule(
		sLog,
		versions,
		job,
		resources,
	)

	metric.SchedulingJobDuration{
		PipelineName: runner.Pipeline.Name(),
		JobName:      job.Name(),
		JobID:        job.ID(),
		Duration:     time.Since(jStart),
	}.Emit(sLog)
}
