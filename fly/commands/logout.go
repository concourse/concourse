package commands

import (
	"errors"
	"fmt"

	"github.com/concourse/fly/rc"
)

type LogoutCommand struct {
	All bool `short:"a" long:"all" description:"Logout of all targets"`
}

func (command *LogoutCommand) Execute(args []string) error {

	if Fly.Target != "" && !command.All {
		if err := rc.DeleteTarget(Fly.Target); err != nil {
			return err
		}

		fmt.Println("logged out of target: " + Fly.Target)
	} else if Fly.Target == "" && command.All {

		flyYAML, err := rc.LoadTargets()
		if err != nil {
			return err
		}

		for targetName := range flyYAML.Targets {
			if err := rc.DeleteTarget(targetName); err != nil {
				return err
			}
		}

		fmt.Println("logged out of all targets")
	} else {
		return errors.New("must specify either --target or --all")
	}

	return nil
}
