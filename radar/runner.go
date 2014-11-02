package radar

import (
	"os"
	"time"

	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/rata"
)

type Locker interface {
	AcquireWriteLockImmediately(lock []db.NamedLock) (db.Lock, error)
	AcquireReadLock(lock []db.NamedLock) (db.Lock, error)
	AcquireWriteLock(lock []db.NamedLock) (db.Lock, error)
}

type Scanner interface {
	Scan(ResourceChecker, string) ifrit.Process
}

type Runner struct {
	logger lager.Logger

	noop bool

	locker   Locker
	scanner  Scanner
	configDB ConfigDB

	syncInterval time.Duration

	turbineEndpoint *rata.RequestGenerator
}

func NewRunner(
	logger lager.Logger,
	noop bool,
	locker Locker,
	scanner Scanner,
	configDB ConfigDB,
	syncInterval time.Duration,
	turbineEndpoint *rata.RequestGenerator,
) *Runner {
	return &Runner{
		logger:          logger,
		noop:            noop,
		locker:          locker,
		scanner:         scanner,
		configDB:        configDB,
		syncInterval:    syncInterval,
		turbineEndpoint: turbineEndpoint,
	}
}

func (runner *Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	if runner.noop {
		<-signals
		return nil
	}

	if runner.logger != nil {
		runner.logger.Info("scanning")
	}

	ticker := time.NewTicker(runner.syncInterval)

	scanning := make(map[string]ifrit.Process)

dance:
	for {
		select {
		case <-signals:
			for _, process := range scanning {
				process.Signal(os.Interrupt)
			}

			break dance

		case <-ticker.C:
			config, err := runner.configDB.GetConfig()
			if err != nil {
				continue
			}

			// reap dead scanners
			alreadyScanning := map[string]bool{}
			for resource, process := range scanning {
				select {
				case <-process.Wait():
					delete(scanning, resource)
				default:
					alreadyScanning[resource] = true
				}
			}

			// start missing scanners
			for _, resource := range config.Resources {
				if alreadyScanning[resource.Name] {
					continue
				}

				checker := NewTurbineChecker(runner.turbineEndpoint)
				scanning[resource.Name] = runner.scanner.Scan(checker, resource.Name)
			}
		}
	}

	return nil
}
