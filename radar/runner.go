package radar

import (
	"os"
	"time"

	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/rata"
)

type Locker interface {
	AcquireWriteLockImmediately(lock []db.NamedLock) (db.Lock, error)
	AcquireReadLock(lock []db.NamedLock) (db.Lock, error)
	AcquireWriteLock(lock []db.NamedLock) (db.Lock, error)
}

type Scanner interface {
	Scan(ResourceChecker, string) ifrit.Runner
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

	scannersGroup := grouper.NewDynamic(nil, 0, 0)

	scannersClient := scannersGroup.Client()
	exits := scannersClient.ExitListener()
	insertScanner := scannersClient.Inserter()

	scanners := ifrit.Invoke(scannersGroup)

	scanning := make(map[string]bool)

dance:
	for {
		select {
		case <-signals:
			scanners.Signal(os.Interrupt)

			// don't bother waiting for scanners on shutdown

			break dance

		case exited := <-exits:
			delete(scanning, exited.Member.Name)

		case <-ticker.C:
			config, err := runner.configDB.GetConfig()
			if err != nil {
				continue
			}

			for _, resource := range config.Resources {
				if scanning[resource.Name] {
					continue
				}

				checker := NewTurbineChecker(runner.turbineEndpoint)

				scanning[resource.Name] = true

				// avoid deadlock if exit event is blocked; inserting in this case
				// will block on the event being consumed (which is in this select)
				go func(name string) {
					insertScanner <- grouper.Member{
						Name:   name,
						Runner: runner.scanner.Scan(checker, name),
					}
				}(resource.Name)
			}
		}
	}

	return nil
}
