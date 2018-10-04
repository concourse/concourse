package commands

import (
	"fmt"
	"os"
	"sort"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
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

	fmt.Println("Team Name:", command.TeamName)

	for _, role := range roles {
		authUsers := authRoles[role]["users"]
		authGroups := authRoles[role]["groups"]

		fmt.Printf("\nUsers (%s):\n", role)
		if len(authUsers) > 0 {
			for _, user := range authUsers {
				fmt.Println("-", user)
			}
		} else {
			fmt.Println("- none")
		}

		fmt.Printf("\nGroups (%s):\n", role)
		if len(authGroups) > 0 {
			for _, group := range authGroups {
				fmt.Println("-", group)
			}
		} else {
			fmt.Println("- none")
		}

		if len(authUsers) == 0 && len(authGroups) == 0 {
			command.WarnAllowAllUsers(role)
		}
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

	team := atc.Team{Auth: atc.TeamAuth(authRoles)}

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

func (command *SetTeamCommand) ErrorAuthNotConfigured(err error) {
	switch err {
	case skycmd.ErrRequireAllowAllUsersConfig:
		fmt.Fprintln(ui.Stderr, "You have not provided a list of users and groups for one of the roles in your config yaml.")

	case skycmd.ErrRequireAllowAllUsersFlag:
		fmt.Fprintln(ui.Stderr, "You have not provided a whitelist of users or groups. To continue, run:")
		fmt.Fprintln(ui.Stderr, "")
		fmt.Fprintln(ui.Stderr, "    "+ui.Embolden("fly -t %s set-team -n %s --allow-all-users", Fly.Target, command.TeamName))
		fmt.Fprintln(ui.Stderr, "")
		fmt.Fprintln(ui.Stderr, "This will allow team access to all logged in users in the system.")
	}
}

func (command *SetTeamCommand) WarnAllowAllUsers(role string) {
	fmt.Fprintln(ui.Stderr, "")
	displayhelpers.PrintWarningHeader()
	fmt.Fprintf(ui.Stderr, ui.WarningColor("Granting role '%s' to ALL users. You asked for it!\n", role))
}
