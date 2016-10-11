package commands

import (
	"errors"

	"github.com/concourse/fly/rc"
)

type LogoutCommand struct {
	All bool `short:"a" long:"all" description:"Logout of all targets"`
}

func (command *LogoutCommand) Execute(args []string) (err error) {

	if Fly.Target != "" && !command.All {
		err = rc.DeleteTarget(Fly.Target)
	} else if Fly.Target == "" && command.All {
		flyYAML, err := rc.LoadTargets()
		if err != nil {
			return err
		}

		for targetName, _ := range flyYAML.Targets {
			if err = rc.DeleteTarget(targetName); err != nil {
				return err
			}
		}

	} else {
		err = errors.New("Must specify either a target (--target/-t) or the all flag (--all/-a)")
	}

	return
}
