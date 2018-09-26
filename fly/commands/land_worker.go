package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/rc"
)

type LandWorkerCommand struct {
	Worker string `short:"w"  long:"worker" required:"true" description:"Worker to land"`
}

func (command *LandWorkerCommand) Execute(args []string) error {
	workerName := command.Worker

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	err = target.Client().LandWorker(workerName)
	if err != nil {
		return err
	}

	fmt.Printf("landed '%s'\n", workerName)

	return nil
}
