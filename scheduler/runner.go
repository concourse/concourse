package scheduler

import (
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Locker

type Locker interface {
	AcquireWriteLock([]db.NamedLock) (db.Lock, error)
	AcquireWriteLockImmediately([]db.NamedLock) (db.Lock, error)

	AcquireReadLock([]db.NamedLock) (db.Lock, error)
}

//go:generate counterfeiter . BuildScheduler

type BuildScheduler interface {
	TryNextPendingBuild(lager.Logger, atc.JobConfig, atc.ResourceConfigs) Waiter
	BuildLatestInputs(lager.Logger, atc.JobConfig, atc.ResourceConfigs) error
}

type Runner struct {
	Logger lager.Logger

	Locker Locker
	DB     db.PipelineDB

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
	logger.Info("start")
	defer logger.Info("done")

	config, _, err := runner.DB.GetConfig()
	if err != nil {
		if err == db.ErrPipelineNotFound {
			return err
		}

		logger.Error("failed-to-get-config", err)

		return nil
	}

	if runner.Noop {
		return nil
	}

	for _, job := range config.Jobs {
		lock := []db.NamedLock{db.JobSchedulingLock(runner.DB.ScopedName(job.Name))}
		jobCheckingLock, err := runner.Locker.AcquireWriteLockImmediately(lock)
		if err != nil {
			continue
		}

		sLog := logger.Session("scheduling", lager.Data{
			"job": job.Name,
		})

		runner.schedule(sLog, job, config.Resources)

		jobCheckingLock.Release()
	}

	return nil
}

func (runner *Runner) schedule(logger lager.Logger, job atc.JobConfig, resources atc.ResourceConfigs) {
	runner.Scheduler.TryNextPendingBuild(logger, job, resources).Wait()

	err := runner.Scheduler.BuildLatestInputs(logger, job, resources)
	if err != nil {
		logger.Error("failed-to-build-from-latest-inputs", err)
	}
}
