package commands

import (
	"fmt"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/concourse/skymarshal/skycmd"
	"github.com/jessevdk/go-flags"
	"github.com/vito/go-interact/interact"
)

func WireTeamConnectors(command *flags.Command) {
	for _, group := range command.Groups() {
		if group.ShortDescription == "Authentication" {
			skycmd.WireTeamConnectors(group)
			return
		}
	}
}

type SetTeamCommand struct {
	TeamName        string               `short:"n" long:"team-name" required:"true" description:"The team to create or modify"`
	SkipInteractive bool                 `long:"non-interactive" description:"Force apply configuration"`
	AuthFlags       skycmd.AuthTeamFlags `group:"Authentication"`
}

func (command *SetTeamCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	auth, err := command.AuthFlags.Format()
	if err != nil {
		command.ErrorAuthNotConfigured()
	}

	fmt.Println("Team Name:", command.TeamName)

	fmt.Println("\nUsers:")
	if len(auth["users"]) > 0 {
		for _, user := range auth["users"] {
			fmt.Println("-", user)
		}
	} else {
		fmt.Println("- none")
	}

	fmt.Println("\nGroups:")
	if len(auth["groups"]) > 0 {
		for _, group := range auth["groups"] {
			fmt.Println("-", group)
		}
	} else {
		fmt.Println("- none")
	}

	if len(auth["users"]) == 0 && len(auth["groups"]) == 0 {
		command.WarnAllowAllUsers()
	}

	confirm := true
	if !command.SkipInteractive {
		confirm = false
		err = interact.NewInteraction("\napply configuration?").Resolve(&confirm)
		if err != nil {
			return err
		}
	}

	if !confirm {
		displayhelpers.Failf("bailing out")
	}

	team := atc.Team{Auth: auth}

	_, created, updated, err := target.Client().Team(command.TeamName).CreateOrUpdate(team)
	if err != nil {
		return err
	}

	if created {
		fmt.Println("team created")
	} else if updated {
		fmt.Println("team updated")
	}

	return nil
}

func (command *SetTeamCommand) ErrorAuthNotConfigured() {
	fmt.Fprintln(ui.Stderr, "You have not provided a whitelist of users or groups. To continue, run:")
	fmt.Fprintln(ui.Stderr, "")
	fmt.Fprintln(ui.Stderr, "    "+ui.Embolden("fly -t %s set-team -n %s --allow-all-users", Fly.Target, command.TeamName))
	fmt.Fprintln(ui.Stderr, "")
	fmt.Fprintln(ui.Stderr, "This will allow team access to all logged in users in the system.")
	os.Exit(1)
}

func (command *SetTeamCommand) WarnAllowAllUsers() {
	if command.AuthFlags.AllowAllUsers {
		fmt.Fprintln(ui.Stderr, "")
		displayhelpers.PrintWarningHeader()
		fmt.Fprintln(ui.Stderr, ui.WarningColor("Allowing all logged in users. You asked for it!"))
	}
}
