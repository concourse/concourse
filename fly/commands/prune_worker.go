package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
)

type PruneWorkerCommand struct {
	Worker     string `short:"w"  long:"worker" description:"Worker to prune"`
	AllStalled bool   `short:"a" long:"all-stalled" description:"Prune all stalled workers"`
}

func (command *PruneWorkerCommand) Execute(args []string) error {
	if command.Worker == "" && !command.AllStalled {
		displayhelpers.Failf("Either a worker name or --all-stalled are required")
	}

	var workersNames []string

	if command.Worker != "" {
		workersNames = append(workersNames, command.Worker)
	}

	target, err := Fly.RetrieveTarget()
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	if command.AllStalled {
		workers, err := target.Client().ListWorkers()
		if err != nil {
			return err
		}
		for _, worker := range workers {
			if worker.State == "stalled" {
				workersNames = append(workersNames, worker.Name)
			}
		}
		if workersNames == nil {
			fmt.Printf(ui.WarningColor("WARNING: No stalled workers found.\n"))
		}
	}

	for _, workerName := range workersNames {
		err = target.Client().PruneWorker(workerName)
		if err != nil {
			return err
		}

		fmt.Printf("pruned '%s'\n", workerName)
	}
	return nil
}
