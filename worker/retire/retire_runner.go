package retire

import (
	"context"
	"os"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/worker"
)

type RetireWorkerCommand struct {
	TSA worker.TSAConfig `group:"TSA Configuration" namespace:"tsa" required:"true"`

	WorkerName string `long:"name" required:"true" description:"The name of the worker you wish to retire."`
	WorkerTeam string `long:"team" description:"The team name of the worker you wish to retire."`
}

func (cmd *RetireWorkerCommand) Execute(args []string) error {
	logger := lager.NewLogger("retire-worker")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	client := cmd.TSA.Client(atc.Worker{
		Name: cmd.WorkerName,
		Team: cmd.WorkerTeam,
	})

	return client.Retire(lagerctx.NewContext(context.Background(), logger))
}
