package commands

import (
	"errors"

	"github.com/concourse/fly/rc"
)

type LogoutCommand struct {
	All bool `short:"a" long:"all" description:"Logout of all targets"`
}

func (command *LogoutCommand) Execute(args []string) error {

	if Fly.Target != "" && !command.All {
		return rc.DeleteTarget(Fly.Target)
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

	} else {
		return errors.New("must specify either --target or --all")
	}

	return nil
}
