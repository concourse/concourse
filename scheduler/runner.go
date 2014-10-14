package scheduler

import (
	"os"
	"time"

	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

type Locker interface {
	AcquireBuildSchedulingLock() (db.Lock, error)
}

type BuildScheduler interface {
	TryNextPendingBuild(config.Job) error
	BuildLatestInputs(config.Job) error
}

type Runner struct {
	Logger lager.Logger

	Locker    Locker
	Scheduler BuildScheduler

	Noop bool
	Jobs config.Jobs

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

	lockAcquired := make(chan db.Lock)
	lockErr := make(chan error)

	go func() {
		lock, err := runner.Locker.AcquireBuildSchedulingLock()
		if err != nil {
			lockErr <- err
		} else {
			lockAcquired <- lock
		}
	}()

	var lock db.Lock

	select {
	case lock = <-lockAcquired:
	case err := <-lockErr:
		return err
	case <-signals:
		return nil
	}

	if runner.Logger != nil {
		runner.Logger.Info("polling", lager.Data{
			"inverval": runner.Interval.String(),
		})
	}

dance:
	for {
		select {
		case <-time.After(runner.Interval):
			for _, job := range runner.Jobs {
				runner.Scheduler.TryNextPendingBuild(job)
				runner.Scheduler.BuildLatestInputs(job)
			}

		case <-signals:
			break dance
		}
	}

	return lock.Release()
}
