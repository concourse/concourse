package commands

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/vito/go-interact/interact"
)

type SetTeamCommand struct {
	TeamName     string               `short:"n" long:"team-name" required:"true"        description:"The team to create or modify"`
	NoAuth       bool                 `long:"no-really-i-dont-want-any-auth"  description:"Ignore warnings about insecure teams"`
	BasicAuth    atc.BasicAuthFlag    `group:"Basic Authentication" namespace:"basic-auth"`
	GitHubAuth   atc.GitHubAuthFlag   `group:"GitHub Authentication" namespace:"github-auth"`
	UAAAuth      atc.UAAAuthFlag      `group:"UAA Authentication" namespace:"uaa-auth"`
	GenericOAuth atc.GenericOAuthFlag `group:"Generic OAuth Authentication" namespace:"generic-oauth"`
}

func (command *SetTeamCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target)
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
	fmt.Println("Basic Auth:", authMethodStatusDescription(command.BasicAuth.IsConfigured()))
	fmt.Println("GitHub Auth:", authMethodStatusDescription(command.GitHubAuth.IsConfigured()))
	fmt.Println("UAA Auth:", authMethodStatusDescription(command.UAAAuth.IsConfigured()))
	fmt.Println("Generic OAuth:", authMethodStatusDescription(command.GenericOAuth.IsConfigured()))

	confirm := false
	err = interact.NewInteraction("apply configuration?").Resolve(&confirm)
	if err != nil {
		return err
	}

	if !confirm {
		displayhelpers.Failf("bailing out")
	}

	team := atc.Team{}

	if command.BasicAuth.IsConfigured() {
		team.BasicAuth = &atc.BasicAuth{
			BasicAuthUsername: command.BasicAuth.Username,
			BasicAuthPassword: command.BasicAuth.Password,
		}
	}

	if command.GitHubAuth.IsConfigured() {
		team.GitHubAuth = &atc.GitHubAuth{
			ClientID:      command.GitHubAuth.ClientID,
			ClientSecret:  command.GitHubAuth.ClientSecret,
			Organizations: command.GitHubAuth.Organizations,
			Users:         command.GitHubAuth.Users,
			AuthURL:       command.GitHubAuth.AuthURL,
			TokenURL:      command.GitHubAuth.TokenURL,
			APIURL:        command.GitHubAuth.APIURL,
		}

		for _, ghTeam := range command.GitHubAuth.Teams {
			team.GitHubAuth.Teams = append(team.GitHubAuth.Teams, atc.GitHubTeam{
				OrganizationName: ghTeam.OrganizationName,
				TeamName:         ghTeam.TeamName,
			})
		}
	}

	if command.UAAAuth.IsConfigured() {
		cfCACert := ""
		if command.UAAAuth.CFCACert != "" {
			cfCACertFileContents, err := ioutil.ReadFile(string(command.UAAAuth.CFCACert))
			if err != nil {
				return err
			}
			cfCACert = string(cfCACertFileContents)
		}

		team.UAAAuth = &atc.UAAAuth{
			ClientID:     command.UAAAuth.ClientID,
			ClientSecret: command.UAAAuth.ClientSecret,
			AuthURL:      command.UAAAuth.AuthURL,
			TokenURL:     command.UAAAuth.TokenURL,
			CFSpaces:     command.UAAAuth.CFSpaces,
			CFURL:        command.UAAAuth.CFURL,
			CFCACert:     cfCACert,
		}
	}

	if command.GenericOAuth.IsConfigured() {
		team.GenericOAuth = &atc.GenericOAuth{
			ClientID:      command.GenericOAuth.ClientID,
			ClientSecret:  command.GenericOAuth.ClientSecret,
			AuthURL:       command.GenericOAuth.AuthURL,
			TokenURL:      command.GenericOAuth.TokenURL,
			DisplayName:   command.GenericOAuth.DisplayName,
			AuthURLParams: command.GenericOAuth.AuthURLParams,
		}
	}

	_, _, _, err = target.Client().Team(command.TeamName).CreateOrUpdate(team)
	if err != nil {
		return err
	}

	fmt.Println("team created")
	return nil
}

func (command *SetTeamCommand) noAuthConfigured() bool {
	if command.BasicAuth.IsConfigured() || command.GitHubAuth.IsConfigured() || command.UAAAuth.IsConfigured() || command.GenericOAuth.IsConfigured() {
		return false
	}
	return true
}

func (command *SetTeamCommand) ValidateFlags() error {
	if command.noAuthConfigured() {
		displayhelpers.PrintWarningHeader()

		if !command.NoAuth {
			fmt.Fprintln(os.Stderr, "no auth methods configured! to continue, run:")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "    "+ui.Embolden("fly -t %s set-team -n %s --no-really-i-dont-want-any-auth", Fly.Target, command.TeamName))
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "this will leave the team open to anyone to mess with!")
			os.Exit(1)
		}

		fmt.Fprintln(os.Stderr, ui.WarningColor("no auth methods configured. you asked for it!"))
	}

	if command.BasicAuth.IsConfigured() {
		err := command.BasicAuth.Validate()
		if err != nil {
			return err
		}
	}

	if command.GitHubAuth.IsConfigured() {
		err := command.GitHubAuth.Validate()
		if err != nil {
			return err
		}
	}

	if command.UAAAuth.IsConfigured() {
		err := command.UAAAuth.Validate()
		if err != nil {
			return err
		}
	}

	if command.GenericOAuth.IsConfigured() {
		err := command.GenericOAuth.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

func authMethodStatusDescription(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}
