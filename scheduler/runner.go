package scheduler

import (
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

type Locker interface {
	AcquireWriteLockImmediately(lock []db.NamedLock) (db.Lock, error)
	AcquireReadLock(lock []db.NamedLock) (db.Lock, error)
}

type BuildScheduler interface {
	TryNextPendingBuild(atc.JobConfig) error
	BuildLatestInputs(atc.JobConfig) error

	TrackInFlightBuilds() error
}

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

	if runner.Noop {
		<-signals
		return nil
	}

	if runner.Interval == 0 {
		panic("unconfigured scheduler interval")
	}

	if runner.Logger != nil {
		runner.Logger.Info("starting", lager.Data{
			"inverval": runner.Interval.String(),
		})
	}

dance:
	for {
		select {
		case <-time.After(runner.Interval):
			if runner.Logger != nil {
				runner.Logger.Info("scheduling")
			}

			config, err := runner.ConfigDB.GetConfig()
			if err != nil {
				continue
			}

			runner.Scheduler.TrackInFlightBuilds()

			for _, job := range config.Jobs {
				lock, err := runner.Locker.AcquireWriteLockImmediately([]db.NamedLock{db.JobSchedulingLock(job.Name)})
				if err != nil {
					continue
				}

				runner.Scheduler.TryNextPendingBuild(job)
				runner.Scheduler.BuildLatestInputs(job)

				lock.Release()
			}

		case <-signals:
			break dance
		}
	}

	return nil
}
