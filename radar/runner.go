package radar

import (
	"os"

	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resources"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
)

type Locker interface {
	AcquireResourceCheckingLock() (db.Lock, error)
	AcquireReadLock(names []string) (db.Lock, error)
	AcquireWriteLock(names []string) (db.Lock, error)
}

type Scanner interface {
	Scan(ResourceChecker, config.Resource)
}

type Runner struct {
	Logger lager.Logger

	Locker  Locker
	Scanner Scanner

	Noop      bool
	Resources config.Resources

	TurbineEndpoint *rata.RequestGenerator
}

func (runner *Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	if runner.Noop {
		<-signals
		return nil
	}

	lockAcquired := make(chan db.Lock)
	lockErr := make(chan error)

	go func() {
		lock, err := runner.Locker.AcquireResourceCheckingLock()
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
		runner.Logger.Info("scanning")
	}

	for _, resource := range runner.Resources {
		checker := resources.NewTurbineChecker(runner.TurbineEndpoint)
		runner.Scanner.Scan(checker, resource)
	}

	<-signals

	return lock.Release()
}
