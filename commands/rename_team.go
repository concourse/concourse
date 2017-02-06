package commands

import (
	"fmt"

	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
)

type RenameTeamCommand struct {
	TeamName string `short:"o"  long:"old-name" required:"true"  description:"Team to rename"`
	Name     string `short:"n"  long:"new-name" required:"true"  description:"Name to set as team name"`
}

func (command *RenameTeamCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	err = target.Client().RenameTeam(command.TeamName, command.Name)
	if err != nil {
		displayhelpers.Failf(err.Error())
		return nil
	}

	fmt.Printf("team successfully renamed to %s\n", command.Name)
	return nil
}
