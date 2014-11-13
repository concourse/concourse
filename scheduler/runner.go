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
	TryNextPendingBuild(atc.JobConfig, atc.ResourceConfigs) error
	BuildLatestInputs(atc.JobConfig, atc.ResourceConfigs) error

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

	if runner.Interval == 0 {
		panic("unconfigured scheduler interval")
	}

	runner.Logger.Info("start", lager.Data{
		"inverval": runner.Interval.String(),
	})

	defer runner.Logger.Info("done")

dance:
	for {
		select {
		case <-time.After(runner.Interval):
			runner.tick()

		case <-signals:
			break dance
		}
	}

	return nil
}

func (runner *Runner) tick() {
	sLog := runner.Logger.Session("tick")

	sLog.Info("start")
	defer sLog.Info("done")

	config, err := runner.ConfigDB.GetConfig()
	if err != nil {
		sLog.Error("failed-to-get-config", err)
		return
	}

	err = runner.Scheduler.TrackInFlightBuilds()
	if err != nil {
		sLog.Error("failed-to-track-in-flight-builds", err)
	}

	if runner.Noop {
		return
	}

	for _, job := range config.Jobs {
		lock, err := runner.Locker.AcquireWriteLockImmediately([]db.NamedLock{db.JobSchedulingLock(job.Name)})
		if err != nil {
			continue
		}

		sLog.Info("scheduling", lager.Data{
			"job": job.Name,
		})

		err = runner.Scheduler.TryNextPendingBuild(job, config.Resources)
		if err != nil {
			sLog.Error("failed-to-try-next-pending-build", err)
		}

		err = runner.Scheduler.BuildLatestInputs(job, config.Resources)
		if err != nil {
			sLog.Error("failed-to-build-from-latest-inputs", err)
		}

		lock.Release()
	}
}
