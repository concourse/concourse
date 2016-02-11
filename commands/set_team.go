package commands

import (
	"errors"
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/internal/displayhelpers"
	"github.com/concourse/fly/rc"
	"github.com/vito/go-interact/interact"
)

type SetTeamCommand struct {
	TeamName string `short:"n" long:"team-name" required:"true"        description:"The team to create or modify"`

	BasicAuth struct {
		Username string `long:"username" description:"Username to use for basic auth."`
		Password string `long:"password" description:"Password to use for basic auth."`
	} `group:"Basic Authentication" namespace:"basic-auth"`

	GitHubAuth struct {
		ClientID      string                       `long:"client-id"     description:"Application client ID for enabling GitHub OAuth."`
		ClientSecret  string                       `long:"client-secret" description:"Application client secret for enabling GitHub OAuth."`
		Organizations []string                     `long:"organization"  description:"GitHub organization whose members will have access." value-name:"ORG"`
		Teams         []flaghelpers.GitHubTeamFlag `long:"team"          description:"GitHub team whose members will have access." value-name:"ORG/TEAM"`
		Users         []string                     `long:"user"          description:"GitHub user to permit access." value-name:"LOGIN"`
	} `group:"GitHub Authentication" namespace:"github-auth"`
}

func (command *SetTeamCommand) Execute([]string) error {
	hasBasicAuth, hasGitHubAuth, err := command.ValidateFlags()
	if err != nil {
		return err
	}

	fmt.Println("Team Name:", command.TeamName)
	fmt.Println("Basic Auth:", authMethodStatusDescription(hasBasicAuth))
	fmt.Println("GitHub Auth:", authMethodStatusDescription(hasGitHubAuth))

	confirm := false
	err = interact.NewInteraction("apply configuration?").Resolve(&confirm)
	if err != nil {
		return err
	}

	if !confirm {
		displayhelpers.Failf("bailing out")
	}

	team := command.GetTeam(hasBasicAuth, hasGitHubAuth)

	client, err := rc.TargetClient(Fly.Target)
	if err != nil {
		return err
	}

	_, _, _, err = client.SetTeam(command.TeamName, team)
	if err != nil {
		return err
	}

	fmt.Println("team created")
	return nil
}

func (command *SetTeamCommand) ValidateFlags() (bool, bool, error) {
	hasBasicAuth := command.BasicAuth.Username != "" || command.BasicAuth.Password != ""
	if hasBasicAuth && (command.BasicAuth.Username == "" || command.BasicAuth.Password == "") {
		return false, false, errors.New("Both username and password are required for basic auth.")
	}
	hasGitHubAuth := command.GitHubAuth.ClientID != "" || command.GitHubAuth.ClientSecret != "" ||
		len(command.GitHubAuth.Organizations) > 0 || len(command.GitHubAuth.Teams) > 0 || len(command.GitHubAuth.Users) > 0
	if hasGitHubAuth {
		if command.GitHubAuth.ClientID == "" || command.GitHubAuth.ClientSecret == "" {
			return false, false, errors.New("Both client-id and client-secret are required for github-auth.")
		}
		if len(command.GitHubAuth.Organizations) == 0 &&
			len(command.GitHubAuth.Teams) == 0 &&
			len(command.GitHubAuth.Users) == 0 {
			return false, false, errors.New("At least one of the following is required for github-auth: organizations, teams, users")
		}
	}

	return hasBasicAuth, hasGitHubAuth, nil
}

func authMethodStatusDescription(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func (command *SetTeamCommand) GetTeam(basicAuthEnabled, gitHubAuthEnabled bool) atc.Team {
	team := atc.Team{}

	if basicAuthEnabled {
		team.BasicAuth.BasicAuthUsername = command.BasicAuth.Username
		team.BasicAuth.BasicAuthPassword = command.BasicAuth.Password
	}

	if gitHubAuthEnabled {
		team.GitHubAuth.ClientID = command.GitHubAuth.ClientID
		team.GitHubAuth.ClientSecret = command.GitHubAuth.ClientSecret
		team.GitHubAuth.Organizations = command.GitHubAuth.Organizations
		team.GitHubAuth.Users = command.GitHubAuth.Users

		for _, ghTeam := range command.GitHubAuth.Teams {
			team.GitHubAuth.Teams = append(team.GitHubAuth.Teams, atc.GitHubTeam{
				OrganizationName: ghTeam.OrganizationName,
				TeamName:         ghTeam.TeamName,
			})
		}
	}

	return team
}
