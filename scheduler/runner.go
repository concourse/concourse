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
	AcquireWriteLockImmediately(lock []db.NamedLock) (db.Lock, error)
	AcquireReadLock(lock []db.NamedLock) (db.Lock, error)
}

//go:generate counterfeiter . BuildScheduler
type BuildScheduler interface {
	TryNextPendingBuild(atc.JobConfig, atc.ResourceConfigs) error
	BuildLatestInputs(atc.JobConfig, atc.ResourceConfigs) error

	TrackInFlightBuilds() error
}

//go:generate counterfeiter . ConfigDB
type ConfigDB interface {
	GetConfig() (atc.Config, error)
}

type Runner struct {
	Logger lager.Logger

	Locker   Locker
	ConfigDB ConfigDB

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
		runner.tick(runner.Logger.Session("tick"))

		select {
		case <-time.After(runner.Interval):
		case <-signals:
			break dance
		}
	}

	return nil
}

func (runner *Runner) tick(logger lager.Logger) {
	logger.Info("start")
	defer logger.Info("done")

	config, err := runner.ConfigDB.GetConfig()
	if err != nil {
		logger.Error("failed-to-get-config", err)
		return
	}

	err = runner.Scheduler.TrackInFlightBuilds()
	if err != nil {
		logger.Error("failed-to-track-in-flight-builds", err)
	}

	if runner.Noop {
		return
	}

	for _, job := range config.Jobs {
		lock, err := runner.Locker.AcquireWriteLockImmediately([]db.NamedLock{db.JobSchedulingLock(job.Name)})
		if err != nil {
			continue
		}

		runner.schedule(job, config.Resources, logger.Session("scheduling", lager.Data{
			"job": job.Name,
		}))

		lock.Release()
	}
}

func (runner *Runner) schedule(job atc.JobConfig, resources atc.ResourceConfigs, logger lager.Logger) {
	err := runner.Scheduler.TryNextPendingBuild(job, resources)
	if err != nil {
		logger.Error("failed-to-try-next-pending-build", err)
	}

	err = runner.Scheduler.BuildLatestInputs(job, resources)
	if err != nil {
		logger.Error("failed-to-build-from-latest-inputs", err)
	}
}
