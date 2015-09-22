package scheduler

import (
	"errors"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/metric"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . BuildScheduler

type BuildScheduler interface {
	TryNextPendingBuild(lager.Logger, *algorithm.VersionsDB, atc.JobConfig, atc.ResourceConfigs) Waiter
	BuildLatestInputs(lager.Logger, *algorithm.VersionsDB, atc.JobConfig, atc.ResourceConfigs) error
}

var errPipelineRemoved = errors.New("pipeline removed")

type Runner struct {
	Logger lager.Logger

	DB db.PipelineDB

	Scheduler BuildScheduler

	Noop bool

	Interval time.Duration
}

func (runner *Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	if runner.Interval == 0 {
		panic("unconfigured scheduler interval")
	}

	runner.Logger.Info("start", lager.Data{
		"inverval": runner.Interval.String(),
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
	config, _, found, err := runner.DB.GetConfig()
	if err != nil {
		logger.Error("failed-to-get-config", err)
		return nil
	}

	if !found {
		return errPipelineRemoved
	}

	if runner.Noop {
		return nil
	}

	schedulingLease, leased, err := runner.DB.LeaseScheduling(runner.Interval)
	if err != nil {
		logger.Error("failed-to-acquire-scheduling-lease", err)
		return nil
	}

	if !leased {
		return nil
	}

	defer schedulingLease.Break()

	start := time.Now()

	defer func() {
		metric.SchedulingFullDuration{
			PipelineName: runner.DB.GetPipelineName(),
			Duration:     time.Since(start),
		}.Emit(logger)
	}()

	versions, err := runner.DB.LoadVersionsDB()
	if err != nil {
		logger.Error("failed-to-load-versions-db", err)
		return err
	}

	metric.SchedulingLoadVersionsDuration{
		PipelineName: runner.DB.GetPipelineName(),
		Duration:     time.Since(start),
	}.Emit(logger)

	for _, job := range config.Jobs {
		sLog := logger.Session("scheduling", lager.Data{
			"job": job.Name,
		})

		jStart := time.Now()

		runner.schedule(sLog, versions, job, config.Resources)

		metric.SchedulingJobDuration{
			PipelineName: runner.DB.GetPipelineName(),
			JobName:      job.Name,
			Duration:     time.Since(jStart),
		}.Emit(sLog)
	}

	return nil
}

func (runner *Runner) schedule(logger lager.Logger, versions *algorithm.VersionsDB, job atc.JobConfig, resources atc.ResourceConfigs) {
	runner.Scheduler.TryNextPendingBuild(logger, versions, job, resources).Wait()

	err := runner.Scheduler.BuildLatestInputs(logger, versions, job, resources)
	if err != nil {
		logger.Error("failed-to-build-from-latest-inputs", err)
	}
}
