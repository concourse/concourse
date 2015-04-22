package radar

import (
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
)

//go:generate counterfeiter . Locker

type Locker interface {
	AcquireWriteLockImmediately(lock []db.NamedLock) (db.Lock, error)
	AcquireReadLock(lock []db.NamedLock) (db.Lock, error)
	AcquireWriteLock(lock []db.NamedLock) (db.Lock, error)
}

//go:generate counterfeiter . ScannerFactory

type ScannerFactory interface {
	Scanner(lager.Logger, string) ifrit.Runner
}

type Runner struct {
	logger lager.Logger

	noop bool

	locker         Locker
	scannerFactory ScannerFactory
	configDB       db.ConfigDB

	syncInterval time.Duration
}

func NewRunner(
	logger lager.Logger,
	noop bool,
	locker Locker,
	scannerFactory ScannerFactory,
	configDB db.ConfigDB,
	syncInterval time.Duration,
) *Runner {
	return &Runner{
		logger:         logger,
		noop:           noop,
		locker:         locker,
		scannerFactory: scannerFactory,
		configDB:       configDB,
		syncInterval:   syncInterval,
	}
}

func (runner *Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	if runner.noop {
		<-signals
		return nil
	}

	runner.logger.Info("start")
	defer runner.logger.Info("done")

	ticker := time.NewTicker(runner.syncInterval)

	scannersGroup := grouper.NewDynamic(nil, 0, 0)

	scannersClient := scannersGroup.Client()
	exits := scannersClient.ExitListener()
	insertScanner := scannersClient.Inserter()

	scanners := ifrit.Invoke(scannersGroup)

	scanning := make(map[string]bool)

	runner.tick(scanning, insertScanner)

dance:
	for {
		select {
		case <-signals:
			scanners.Signal(os.Interrupt)

			// don't bother waiting for scanners on shutdown

			break dance

		case exited := <-exits:
			runner.logger.Error("scanner-exited", exited.Err, lager.Data{
				"member": exited.Member.Name,
			})

			delete(scanning, exited.Member.Name)

		case <-ticker.C:
			runner.tick(scanning, insertScanner)
		}
	}

	return nil
}

func (runner *Runner) tick(scanning map[string]bool, insertScanner chan<- grouper.Member) {
	config, _, err := runner.configDB.GetConfig(atc.DefaultPipelineName)
	if err != nil {
		return
	}

	for _, resource := range config.Resources {
		if scanning[resource.Name] {
			continue
		}

		scanning[resource.Name] = true

		runner := runner.scannerFactory.Scanner(runner.logger.Session("scan", lager.Data{"resource": resource.Name}), resource.Name)

		// avoid deadlock if exit event is blocked; inserting in this case
		// will block on the event being consumed (which is in this select)
		go func(name string) {
			insertScanner <- grouper.Member{
				Name:   name,
				Runner: runner,
			}
		}(resource.Name)
	}
}
