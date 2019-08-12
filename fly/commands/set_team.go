package commands

import (
	"fmt"
	"os"
	"sort"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/concourse/concourse/skymarshal/skycmd"
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
	Team            flaghelpers.TeamFlag `short:"n" long:"team-name" required:"true" description:"The team to create or modify"`
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

	authRoles, err := command.AuthFlags.Format()
	if err != nil {
		command.ErrorAuthNotConfigured(err)
		os.Exit(1)
	}

	roles := []string{}
	for role := range authRoles {
		roles = append(roles, role)
	}
	sort.Strings(roles)

	teamName := command.Team.Name()
	fmt.Println("setting team:", ui.Embolden("%s", teamName))

	for _, role := range roles {
		authUsers := authRoles[role]["users"]
		authGroups := authRoles[role]["groups"]

		fmt.Println()
		fmt.Printf("role %s:\n", ui.Embolden(role))
		fmt.Printf("  users:\n")
		if len(authUsers) > 0 {
			for _, user := range authUsers {
				fmt.Printf("  - %s\n", user)
			}
		} else {
			fmt.Printf("    %s\n", ui.OffColor.Sprint("none"))
		}

		fmt.Println()
		fmt.Printf("  groups:\n")
		if len(authGroups) > 0 {
			for _, group := range authGroups {
				fmt.Printf("  - %s\n", group)
			}
		} else {
			fmt.Printf("    %s\n", ui.OffColor.Sprint("none"))
		}
	}

	confirm := true
	if !command.SkipInteractive {
		confirm = false
		err = interact.NewInteraction("\napply team configuration?").Resolve(&confirm)
		if err != nil {
			return err
		}
	}

	if !confirm {
		displayhelpers.Failf("bailing out")
	}

	team := atc.Team{Auth: atc.TeamAuth(authRoles)}

	_, created, updated, err := target.Client().Team(teamName).CreateOrUpdate(team)
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

func (command *SetTeamCommand) ErrorAuthNotConfigured(err error) {
	switch err {
	case skycmd.ErrAuthNotConfiguredFromFile:
		fmt.Fprintln(ui.Stderr, "You have not provided a list of users and groups for one of the roles in your config yaml.")

	case skycmd.ErrAuthNotConfiguredFromFlags:
		fmt.Fprintln(ui.Stderr, "You have not provided a list of users and groups for the specified team.")

	default:
		fmt.Fprintln(ui.Stderr, "error:", err)
	}
}
