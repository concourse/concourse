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
	TeamName       string        `short:"n" long:"team-name" required:"true"        description:"The team to create or modify"`
	Authentication atc.AuthFlags `group:"Authentication"`
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
	fmt.Println("Basic Auth:", authMethodStatusDescription(command.Authentication.BasicAuth.IsConfigured()))
	fmt.Println("GitHub Auth:", authMethodStatusDescription(command.Authentication.GitHubAuth.IsConfigured()))
	fmt.Println("UAA Auth:", authMethodStatusDescription(command.Authentication.UAAAuth.IsConfigured()))
	fmt.Println("Generic OAuth:", authMethodStatusDescription(command.Authentication.GenericOAuth.IsConfigured()))

	confirm := false
	err = interact.NewInteraction("apply configuration?").Resolve(&confirm)
	if err != nil {
		return err
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

	if command.Authentication.GitHubAuth.IsConfigured() {
		team.GitHubAuth = &atc.GitHubAuth{
			ClientID:      command.Authentication.GitHubAuth.ClientID,
			ClientSecret:  command.Authentication.GitHubAuth.ClientSecret,
			Organizations: command.Authentication.GitHubAuth.Organizations,
			Users:         command.Authentication.GitHubAuth.Users,
			AuthURL:       command.Authentication.GitHubAuth.AuthURL,
			TokenURL:      command.Authentication.GitHubAuth.TokenURL,
			APIURL:        command.Authentication.GitHubAuth.APIURL,
		}

		for _, ghTeam := range command.Authentication.GitHubAuth.Teams {
			team.GitHubAuth.Teams = append(team.GitHubAuth.Teams, atc.GitHubTeam{
				OrganizationName: ghTeam.OrganizationName,
				TeamName:         ghTeam.TeamName,
			})
		}
	}

	if command.Authentication.UAAAuth.IsConfigured() {
		cfCACert := ""
		if command.Authentication.UAAAuth.CFCACert != "" {
			cfCACertFileContents, err := ioutil.ReadFile(string(command.Authentication.UAAAuth.CFCACert))
			if err != nil {
				return err
			}
			cfCACert = string(cfCACertFileContents)
		}

		team.UAAAuth = &atc.UAAAuth{
			ClientID:     command.Authentication.UAAAuth.ClientID,
			ClientSecret: command.Authentication.UAAAuth.ClientSecret,
			AuthURL:      command.Authentication.UAAAuth.AuthURL,
			TokenURL:     command.Authentication.UAAAuth.TokenURL,
			CFSpaces:     command.Authentication.UAAAuth.CFSpaces,
			CFURL:        command.Authentication.UAAAuth.CFURL,
			CFCACert:     cfCACert,
		}
	}

	if command.Authentication.GenericOAuth.IsConfigured() {
		team.GenericOAuth = &atc.GenericOAuth{
			ClientID:      command.Authentication.GenericOAuth.ClientID,
			ClientSecret:  command.Authentication.GenericOAuth.ClientSecret,
			AuthURL:       command.Authentication.GenericOAuth.AuthURL,
			TokenURL:      command.Authentication.GenericOAuth.TokenURL,
			DisplayName:   command.Authentication.GenericOAuth.DisplayName,
			AuthURLParams: command.Authentication.GenericOAuth.AuthURLParams,
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
	if command.Authentication.BasicAuth.IsConfigured() || command.Authentication.GitHubAuth.IsConfigured() || command.Authentication.UAAAuth.IsConfigured() || command.Authentication.GenericOAuth.IsConfigured() {
		return false
	}
	return true
}

func (command *SetTeamCommand) ValidateFlags() error {
	if command.noAuthConfigured() {
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

	if command.Authentication.BasicAuth.IsConfigured() {
		err := command.Authentication.BasicAuth.Validate()
		if err != nil {
			return err
		}
	}

	if command.Authentication.GitHubAuth.IsConfigured() {
		err := command.Authentication.GitHubAuth.Validate()
		if err != nil {
			return err
		}
	}

	if command.Authentication.UAAAuth.IsConfigured() {
		err := command.Authentication.UAAAuth.Validate()
		if err != nil {
			return err
		}
	}

	if command.Authentication.GenericOAuth.IsConfigured() {
		err := command.Authentication.GenericOAuth.Validate()
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
