package commands

import (
	"fmt"

	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
)

type RenameTeamCommand struct {
	TeamName    string `short:"o" long:"old-name" required:"true" description:"Current team name"`
	NewTeamName string `short:"n" long:"new-name" required:"true" description:"New team name"`
}

func (command *RenameTeamCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	found, err := target.Team().RenameTeam(command.TeamName, command.NewTeamName)
	if err != nil {
		return err
	}

	if !found {
		displayhelpers.Failf("Team '%s' not found\n", command.TeamName)
		return nil
	}

	fmt.Printf("Team successfully renamed to %s\n", command.NewTeamName)

	return nil
}
