package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/concourse/go-concourse/concourse"
	"github.com/vito/go-interact/interact"
)

type DestroyTeamCommand struct {
	TeamName        string `short:"n" long:"team-name" required:"true"        description:"The team to delete"`
	SkipInteractive bool   `long:"non-interactive"        description:"Force apply configuration"`
}

func (command *DestroyTeamCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	teamName := command.TeamName
	fmt.Printf("!!! this will remove all data for team `%s`\n\n", teamName)

	if !command.SkipInteractive {
		var confirm string
		err = interact.NewInteraction("please type the team name to confirm").Resolve(interact.Required(&confirm))
		if err != nil {
			return err
		}

		if confirm != teamName {
			return errors.New("incorrect team name; bailing out")
		}
	}

	err = target.Team().DestroyTeam(teamName)
	switch err {
	case nil:
		fmt.Println()
		fmt.Printf("`%s` deleted\n", teamName)
		return nil
	case concourse.ErrDestroyRefused:
		fmt.Println()
		fmt.Printf(ui.WarningColor("could not destroy `%s`\n", teamName))
		fmt.Println()
		fmt.Println("either your team is not an admin or it is the last admin team")
		os.Exit(1)
	default:
		return err
	}

	panic("unreachable")
}
