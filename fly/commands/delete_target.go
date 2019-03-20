package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/rc"
)

type DeleteTargetCommand struct {
	All bool `short:"a" long:"all" description:"Delete all targets"`
}

func (command *DeleteTargetCommand) Execute(args []string) error {
	_, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	if command.All {
		if err := rc.DeleteAllTargets(); err != nil {
			return err
		}

		fmt.Println("deleted all targets")
	} else {
		if err := rc.DeleteTarget(Fly.Target); err != nil {
			return err
		}

		fmt.Println("deleted target: " + Fly.Target)
	}

	return nil
}
