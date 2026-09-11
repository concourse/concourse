package land

import (
	"context"
	"os"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerctx"
	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/worker"
)

type LandWorkerCommand struct {
	TSA worker.TSAConfig `group:"TSA Configuration" namespace:"tsa" required:"true"`

	WorkerName string `long:"name" required:"true" description:"The name of the worker you wish to land."`
}

func (cmd *LandWorkerCommand) Execute(args []string) error {
	logger := lager.NewLogger("land-worker")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	client := cmd.TSA.Client(atc.Worker{
		Name: cmd.WorkerName,
	})

	return client.Land(lagerctx.NewContext(context.Background(), logger))
}
