package scheduler

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/metric"
)

//go:generate counterfeiter . BuildScheduler

type BuildScheduler interface {
	Schedule(
		logger lager.Logger,
		versions *algorithm.VersionsDB,
		jobConfigs atc.JobConfigs,
		resourceConfigs atc.ResourceConfigs,
		resourceTypes atc.VersionedResourceTypes,
	) (map[string]time.Duration, error)

	TriggerImmediately(
		logger lager.Logger,
		jobConfig atc.JobConfig,
		resourceConfigs atc.ResourceConfigs,
		resourceTypes atc.VersionedResourceTypes,
	) (db.Build, Waiter, error)

	SaveNextInputMapping(logger lager.Logger, job atc.JobConfig) error
}

var errPipelineRemoved = errors.New("pipeline removed")

type Runner struct {
	Logger lager.Logger

	DB db.PipelineDB

	Pipeline dbng.Pipeline

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

	schedulingLock, acquired, err := runner.DB.AcquireSchedulingLock(logger, runner.Interval)
	if err != nil {
		logger.Error("failed-to-acquire-scheduling-lock", err)
		return nil
	}

	if !acquired {
		return nil
	}

	defer schedulingLock.Release()

	start := time.Now()

	defer func() {
		metric.SchedulingFullDuration{
			PipelineName: runner.DB.GetPipelineName(),
			Duration:     time.Since(start),
		}.Emit(logger)
	}()

	versions, err := runner.Pipeline.LoadVersionsDB()
	if err != nil {
		logger.Error("failed-to-load-versions-db", err)
		return err
	}

	metric.SchedulingLoadVersionsDuration{
		PipelineName: runner.DB.GetPipelineName(),
		Duration:     time.Since(start),
	}.Emit(logger)

	found, err := runner.DB.Reload()
	if err != nil {
		logger.Error("failed-to-update-pipeline-config", err)
		return nil
	}

	if !found {
		return errPipelineRemoved
	}

	config := runner.DB.Config()

	sLog := logger.Session("scheduling")

	resourceTypes, err := runner.Pipeline.ResourceTypes()
	if err != nil {
		logger.Error("failed-to-get-resource-types", err)
		return err
	}

	schedulingTimes, err := runner.Scheduler.Schedule(
		sLog,
		versions,
		config.Jobs,
		config.Resources,
		deserializeVersionedResourceTypes(resourceTypes),
	)

	for jobName, duration := range schedulingTimes {
		metric.SchedulingJobDuration{
			PipelineName: runner.DB.GetPipelineName(),
			JobName:      jobName,
			Duration:     duration,
		}.Emit(sLog)
	}

	return err
}

func deserializeVersionedResourceTypes(types []dbng.ResourceType) atc.VersionedResourceTypes {
	var versionedResourceTypes atc.VersionedResourceTypes

	for _, t := range types {
		versionedResourceTypes = append(versionedResourceTypes, atc.VersionedResourceType{
			ResourceType: atc.ResourceType{
				Name:   t.Name(),
				Type:   t.Type(),
				Source: t.Source(),
			},
			Version: t.Version(),
		})
	}

	return versionedResourceTypes
}
