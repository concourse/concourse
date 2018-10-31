package retire

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/worker"
	"github.com/concourse/concourse/worker/beacon"
	"github.com/concourse/concourse/worker/tsa"
)

type RetireWorkerCommand struct {
	TSA tsa.Config `group:"TSA Configuration" namespace:"tsa" required:"true"`

	WorkerName string `long:"name" required:"true" description:"The name of the worker you wish to retire."`
}

func (cmd *RetireWorkerCommand) Execute(args []string) error {
	logger := lager.NewLogger("retire-worker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	beacon := worker.NewBeacon(
		logger,
		atc.Worker{
			Name: cmd.WorkerName,
		},
		beacon.Config{
			TSAConfig: cmd.TSA,
		},
	)

	return beacon.RetireWorker()
}
