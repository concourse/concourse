package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/concourse/skymarshal/provider"
	"github.com/vito/go-interact/interact"
)

type SetTeamCommand struct {
	TeamName        string `short:"n" long:"team-name" required:"true"        description:"The team to create or modify"`
	SkipInteractive bool   `long:"non-interactive" description:"Force apply configuration"`

	Auth struct {
		Configs provider.AuthConfigs
	} `group:"Authentication"`
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
	fmt.Println("Basic Auth:", authMethodStatusDescription(command.Auth.Configs["basicauth"].IsConfigured()))
	fmt.Println("Bitbucket Cloud Auth:", authMethodStatusDescription(command.Auth.Configs["bitbucket-cloud"].IsConfigured()))
	fmt.Println("Bitbucket Server Auth:", authMethodStatusDescription(command.Auth.Configs["bitbucket-server"].IsConfigured()))
	fmt.Println("GitHub Auth:", authMethodStatusDescription(command.Auth.Configs["github"].IsConfigured()))
	fmt.Println("GitLab Auth:", authMethodStatusDescription(command.Auth.Configs["gitlab"].IsConfigured()))
	fmt.Println("UAA Auth:", authMethodStatusDescription(command.Auth.Configs["uaa"].IsConfigured()))
	fmt.Println("Generic OAuth:", authMethodStatusDescription(command.Auth.Configs["oauth"].IsConfigured()))
	fmt.Println("Generic OAuth OIDC:", authMethodStatusDescription(command.Auth.Configs["oauth_oidc"].IsConfigured()))

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

	providers := provider.GetProviders()
	teamAuth := make(map[string]*json.RawMessage)

	for name, config := range command.Auth.Configs {
		if config.IsConfigured() {

			p, found := providers[name]
			if !found {
				return errors.New("provider not found: " + name)
			}

			data, err := p.MarshalConfig(config)
			if err != nil {
				return err
			}

			teamAuth[name] = data
		}
	}

	if len(teamAuth) > 1 {
		delete(teamAuth, "noauth")
	}

	team := atc.Team{}
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
	configured := 0

	for _, p := range command.Auth.Configs {
		if p.IsConfigured() {
			err := p.Validate()

			if err != nil {
				return err
			}
			configured += 1
		}
	}

	if configured == 0 {
		fmt.Fprintln(ui.Stderr, "no auth methods configured! to continue, run:")
		fmt.Fprintln(ui.Stderr, "")
		fmt.Fprintln(ui.Stderr, "    "+ui.Embolden("fly -t %s set-team -n %s --no-really-i-dont-want-any-auth", Fly.Target, command.TeamName))
		fmt.Fprintln(ui.Stderr, "")
		fmt.Fprintln(ui.Stderr, "this will leave the team open to anyone to mess with!")
		os.Exit(1)
	}

	if configured == 1 && command.Auth.Configs["noauth"].IsConfigured() {
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
