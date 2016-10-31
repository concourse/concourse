package commands

import (
	"errors"
	"fmt"

	"github.com/concourse/fly/rc"
	"github.com/concourse/go-concourse/concourse"
	"github.com/vito/go-interact/interact"
)

type DestroyTeamCommand struct {
	TeamName string `short:"n" long:"team-name" required:"true"        description:"The team to delete"`
}

func (command *DestroyTeamCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	teamName := command.TeamName
	fmt.Printf("!!! this will remove all data for team `%s`\n\n", teamName)

	var confirm string
	err = interact.NewInteraction("are you sure? please type the team name to continue:").Resolve(&confirm)
	if err != nil {
		return err
	}
	if confirm != teamName {
		return errors.New("you typed in the team name incorrectly, bailing out")
	}

	err = target.Team().DestroyTeam(teamName)
	switch err {
	case nil:
		fmt.Printf("%s has been destroyed\n", teamName)
		return nil
	case concourse.ErrDestroyRefused:
		fmt.Printf("only admin can delete teams\n")
		return err
	default:
		fmt.Printf("delete failed due to server error\n")
		return err
	}
}
