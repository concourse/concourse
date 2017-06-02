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
	pipeline          db.Pipeline
	syncInterval      time.Duration
}

func NewRunner(
	logger lager.Logger,
	noop bool,
	scanRunnerFactory ScanRunnerFactory,
	pipeline db.Pipeline,
	syncInterval time.Duration,
) *Runner {
	return &Runner{
		logger:            logger,
		noop:              noop,
		scanRunnerFactory: scanRunnerFactory,
		pipeline:          pipeline,
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
			delete(scanningResourceTypes, exited.Member.Name)

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
	found, err := runner.pipeline.Reload()
	if err != nil {
		runner.logger.Error("failed-to-reload-pipeline", err)
		return
	}

	if !found {
		runner.logger.Info("pipeline-removed")
		return
	}

	config, _, _, err := runner.pipeline.Config()
	if err != nil {
		runner.logger.Error("failed-to-get-config", err)
		return
	}

	for _, resourceType := range config.ResourceTypes {
		scopedName := runner.pipeline.ScopedName("resource-type:" + resourceType.Name)

		if scanningResourceTypes[scopedName] {
			continue
		}

		scanningResourceTypes[scopedName] = true

		logger := runner.logger.Session("scan-resource-type", lager.Data{
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
		scopedName := runner.pipeline.ScopedName("resource:" + resource.Name)

		if scanning[scopedName] {
			continue
		}

		scanning[scopedName] = true

		logger := runner.logger.Session("scan-resource", lager.Data{
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
