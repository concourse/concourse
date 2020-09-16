package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
)

type RenameTeamCommand struct {
	Team        flaghelpers.TeamFlag `short:"o" long:"old-name" required:"true" description:"Current team name"`
	NewTeamName string               `short:"n" long:"new-name" required:"true" description:"New team name"`
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

	teamName := command.Team.Name()

	found, warnings, err := target.Team().RenameTeam(teamName, command.NewTeamName)
	if err != nil {
		return err
	}

	if len(warnings) > 0 {
		displayhelpers.ShowWarnings(warnings)
	}

	if !found {
		displayhelpers.Failf("Team '%s' not found\n", teamName)
		return nil
	}

	fmt.Printf("Team successfully renamed to %s\n", command.NewTeamName)

	return nil
}
