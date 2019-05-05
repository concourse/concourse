package scheduler

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/algorithm"
	"github.com/concourse/concourse/metrics"
)

//go:generate counterfeiter . BuildScheduler

type BuildScheduler interface {
	Schedule(
		logger lager.Logger,
		versions *algorithm.VersionsDB,
		jobs []db.Job,
		resources db.Resources,
		resourceTypes atc.VersionedResourceTypes,
	) (map[string]time.Duration, error)
}

var errPipelineRemoved = errors.New("pipeline removed")

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
		start := time.Now()
		err := runner.tick(runner.Logger.Session("tick"))

		metrics.
			SchedulingDuration.
			WithLabelValues(metrics.StatusFromError(err)).
			Observe(time.Since(start).Seconds())

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

	schedulingLock, acquired, err := runner.Pipeline.AcquireSchedulingLock(logger, runner.Interval)
	if err != nil {
		logger.Error("failed-to-acquire-scheduling-lock", err)
		return nil
	}

	if !acquired {
		return nil
	}

	defer schedulingLock.Release()

	versions, err := runner.Pipeline.LoadVersionsDB()
	if err != nil {
		logger.Error("failed-to-load-versions-db", err)
		return err
	}

	found, err := runner.Pipeline.Reload()
	if err != nil {
		logger.Error("failed-to-update-pipeline-config", err)
		return nil
	}

	if !found {
		return errPipelineRemoved
	}

	resources, err := runner.Pipeline.Resources()
	if err != nil {
		logger.Error("failed-to-get-resources", err)
		return err
	}

	jobs, err := runner.Pipeline.Jobs()
	if err != nil {
		logger.Error("failed-to-get-jobs", err)
		return err
	}

	resourceTypes, err := runner.Pipeline.ResourceTypes()
	if err != nil {
		logger.Error("failed-to-get-resource-types", err)
		return err
	}

	sLog := logger.Session("scheduling")
	_, err = runner.Scheduler.Schedule(
		sLog,
		versions,
		jobs,
		resources,
		resourceTypes.Deserialize(),
	)

	return err
}
