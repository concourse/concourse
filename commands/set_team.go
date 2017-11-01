package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/vito/go-interact/interact"
)

type SetTeamCommand struct {
	TeamName        string        `short:"n" long:"team-name" required:"true"        description:"The team to create or modify"`
	SkipInteractive bool          `long:"non-interactive" description:"Force apply configuration"`
	Authentication  atc.AuthFlags `group:"Authentication"`

	ProviderAuth map[string]provider.AuthConfig
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

	err = command.ValidateFlags()
	if err != nil {
		return err
	}

	fmt.Println("Team Name:", command.TeamName)
	fmt.Println("Basic Auth:", authMethodStatusDescription(command.Authentication.BasicAuth.IsConfigured()))
	fmt.Println("Bitbucket Cloud Auth:", authMethodStatusDescription(command.ProviderAuth["bitbucket-cloud"].IsConfigured()))
	fmt.Println("Bitbucket Server Auth:", authMethodStatusDescription(command.ProviderAuth["bitbucket-server"].IsConfigured()))
	fmt.Println("GitHub Auth:", authMethodStatusDescription(command.ProviderAuth["github"].IsConfigured()))
	fmt.Println("GitLab Auth:", authMethodStatusDescription(command.ProviderAuth["gitlab"].IsConfigured()))
	fmt.Println("UAA Auth:", authMethodStatusDescription(command.ProviderAuth["uaa"].IsConfigured()))
	fmt.Println("Generic OAuth:", authMethodStatusDescription(command.ProviderAuth["oauth"].IsConfigured()))

	confirm := true
	if !command.SkipInteractive {
		confirm = false
		err = interact.NewInteraction("apply configuration?").Resolve(&confirm)
		if err != nil {
			return err
		}
	}

	if !confirm {
		displayhelpers.Failf("bailing out")
	}

	team := atc.Team{}

	if command.Authentication.BasicAuth.IsConfigured() {
		team.BasicAuth = &atc.BasicAuth{
			BasicAuthUsername: command.Authentication.BasicAuth.Username,
			BasicAuthPassword: command.Authentication.BasicAuth.Password,
		}
	}

	teamAuth := make(map[string]*json.RawMessage)
	for name, config := range command.ProviderAuth {
		if config.IsConfigured() {
			data, err := json.Marshal(config)
			if err != nil {
				return err
			}

			teamAuth[name] = (*json.RawMessage)(&data)
		}
	}
	team.Auth = teamAuth

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

func (command *SetTeamCommand) ValidateFlags() error {
	isConfigured := false

	if command.Authentication.BasicAuth.IsConfigured() {
		err := command.Authentication.BasicAuth.Validate()
		if err != nil {
			return err
		}
		isConfigured = true
	}

	for _, p := range command.ProviderAuth {
		if p.IsConfigured() {
			err := p.Validate()

			if err != nil {
				return err
			}
			isConfigured = true
		}
	}

	if !isConfigured {
		if !command.Authentication.NoAuth {
			fmt.Fprintln(ui.Stderr, "no auth methods configured! to continue, run:")
			fmt.Fprintln(ui.Stderr, "")
			fmt.Fprintln(ui.Stderr, "    "+ui.Embolden("fly -t %s set-team -n %s --no-really-i-dont-want-any-auth", Fly.Target, command.TeamName))
			fmt.Fprintln(ui.Stderr, "")
			fmt.Fprintln(ui.Stderr, "this will leave the team open to anyone to mess with!")
			os.Exit(1)
		}

		displayhelpers.PrintWarningHeader()
		fmt.Fprintln(ui.Stderr, ui.WarningColor("no auth methods configured. you asked for it!"))
		fmt.Fprintln(ui.Stderr, "")
	}

	return nil
}

func authMethodStatusDescription(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}
