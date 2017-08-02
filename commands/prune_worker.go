package commands

import (
	"fmt"

	"github.com/concourse/fly/rc"
)

type PruneWorkerCommand struct {
	Worker string `short:"w"  long:"worker" required:"true" description:"Worker to prune"`
}

func (command *PruneWorkerCommand) Execute(args []string) error {
	workerName := command.Worker

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	err = target.Client().PruneWorker(workerName)
	if err != nil {
		return err
	}

	fmt.Printf("pruned '%s'\n", workerName)

	return nil
}
