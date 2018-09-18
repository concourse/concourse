package land

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/worker"
	"github.com/concourse/worker/beacon"
	"github.com/concourse/worker/tsa"
	"github.com/tedsuo/ifrit"
)

type LandWorkerCommand struct {
	TSA tsa.Config `group:"TSA Configuration" namespace:"tsa" required:"true"`

	WorkerName string `long:"name" required:"true" description:"The name of the worker you wish to land."`
}

func (cmd *LandWorkerCommand) Execute(args []string) error {
	logger := lager.NewLogger("land-worker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	landWorkerRunner := cmd.landWorkerRunner(logger)

	return <-ifrit.Invoke(landWorkerRunner).Wait()
}

func (cmd *LandWorkerCommand) landWorkerRunner(logger lager.Logger) ifrit.Runner {
	beacon := worker.NewBeacon(
		logger,
		atc.Worker{
			Name: cmd.WorkerName,
		},
		beacon.Config{
			TSAConfig: cmd.TSA,
		},
	)

	return ifrit.RunFunc(beacon.LandWorker)
}
