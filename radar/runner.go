package radar

import (
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
)

//go:generate counterfeiter . ScanRunnerFactory

type Runner struct {
	logger lager.Logger

	noop bool

	scanRunnerFactory ScanRunnerFactory
	db                db.PipelineDB
	pipelineDBFactory db.PipelineDBFactory
	syncInterval      time.Duration
}

func NewRunner(
	logger lager.Logger,
	noop bool,
	scanRunnerFactory ScanRunnerFactory,
	db db.PipelineDB,
	syncInterval time.Duration,
) *Runner {
	return &Runner{
		logger:            logger,
		noop:              noop,
		scanRunnerFactory: scanRunnerFactory,
		db:                db,
		syncInterval:      syncInterval,
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
	scanningResourceTypes := make(map[string]bool)

	runner.tick(scanning, scanningResourceTypes, insertScanner)

dance:
	for {
		select {
		case <-signals:
			scanners.Signal(os.Interrupt)

			// don't bother waiting for scanners on shutdown

			break dance

		case exited := <-exits:
			if exited.Err != nil {
				runner.logger.Error("scanner-failed", exited.Err, lager.Data{
					"member": exited.Member.Name,
				})
			} else {
				runner.logger.Info("scanner-exited", lager.Data{
					"member": exited.Member.Name,
				})
			}

			delete(scanning, exited.Member.Name)

		case <-ticker.C:
			runner.tick(scanning, scanningResourceTypes, insertScanner)
		}
	}

	return nil
}

func (runner *Runner) tick(
	scanning map[string]bool,
	scanningResourceTypes map[string]bool,
	insertScanner chan<- grouper.Member,
) {
	config, _, found, err := runner.db.GetConfig()
	if err != nil {
		runner.logger.Error("failed-to-get-config", err)
		return
	}

	if !found {
		runner.logger.Info("pipeline-removed")
		return
	}

	for _, resourceType := range config.ResourceTypes {
		scopedName := runner.db.ScopedName("resource-type:" + resourceType.Name)

		if scanningResourceTypes[scopedName] {
			continue
		}

		scanningResourceTypes[scopedName] = true

		logger := runner.logger.Session("scan", lager.Data{
			"pipeline-scoped-name": scopedName,
		})
		runner := runner.scanRunnerFactory.ScanResourceTypeRunner(logger, resourceType.Name)

		// avoid deadlock if exit event is blocked; inserting in this case
		// will block on the event being consumed (which is in this select)
		go func(name string) {
			insertScanner <- grouper.Member{
				Name:   name,
				Runner: runner,
			}
		}(scopedName)
	}

	for _, resource := range config.Resources {
		scopedName := runner.db.ScopedName("resource:" + resource.Name)

		if scanning[scopedName] {
			continue
		}

		scanning[scopedName] = true

		logger := runner.logger.Session("scan", lager.Data{
			"pipeline-scoped-name": scopedName,
		})
		runner := runner.scanRunnerFactory.ScanResourceRunner(logger, resource.Name)

		// avoid deadlock if exit event is blocked; inserting in this case
		// will block on the event being consumed (which is in this select)
		go func(name string) {
			insertScanner <- grouper.Member{
				Name:   name,
				Runner: runner,
			}
		}(scopedName)
	}

}
