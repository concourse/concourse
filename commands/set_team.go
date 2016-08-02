package commands

import (
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/commands/internal/flaghelpers"
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

	UAAAuth UAAAuth `group:"UAA Authentication" namespace:"uaa-auth"`
}

type UAAAuth struct {
	ClientID     string               `long:"client-id"     description:"Application client ID for enabling UAA OAuth."`
	ClientSecret string               `long:"client-secret" description:"Application client secret for enabling UAA OAuth."`
	AuthURL      string               `long:"auth-url"      description:"UAA AuthURL endpoint."`
	TokenURL     string               `long:"token-url"     description:"UAA TokenURL endpoint."`
	CFSpaces     []string             `long:"cf-space"      description:"Space GUID for a CF space whose developers will have access."`
	CFURL        string               `long:"cf-url"        description:"CF API endpoint."`
	CFCACert     flaghelpers.PathFlag `long:"cf-ca-cert"    description:"Path to CF PEM-encoded CA certificate file."`
}

func (auth *UAAAuth) IsConfigured() bool {
	return auth.ClientID != "" ||
		auth.ClientSecret != "" ||
		len(auth.CFSpaces) > 0 ||
		auth.AuthURL != "" ||
		auth.TokenURL != "" ||
		auth.CFURL != ""
}

func (auth *UAAAuth) Validate() error {
	if auth.ClientID == "" || auth.ClientSecret == "" {
		return errors.New("Both client-id and client-secret are required for uaa-auth.")
	}
	if len(auth.CFSpaces) == 0 {
		return errors.New("cf-space is required for uaa-auth.")
	}
	if auth.AuthURL == "" || auth.TokenURL == "" || auth.CFURL == "" {
		return errors.New("auth-url, token-url and cf-url are required for uaa-auth.")
	}
	return nil
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

	hasBasicAuth, hasGitHubAuth, err := command.ValidateFlags()
	if err != nil {
		return err
	}

	fmt.Println("Team Name:", command.TeamName)
	fmt.Println("Basic Auth:", authMethodStatusDescription(hasBasicAuth))
	fmt.Println("GitHub Auth:", authMethodStatusDescription(hasGitHubAuth))
	fmt.Println("UAA Auth:", authMethodStatusDescription(command.UAAAuth.IsConfigured()))

	confirm := false
	err = interact.NewInteraction("apply configuration?").Resolve(&confirm)
	if err != nil {
		return err
	}

	if !confirm {
		displayhelpers.Failf("bailing out")
	}

	team := atc.Team{}

	if hasBasicAuth {
		team.BasicAuth = &atc.BasicAuth{
			BasicAuthUsername: command.BasicAuth.Username,
			BasicAuthPassword: command.BasicAuth.Password,
		}
	}

	if hasGitHubAuth {
		team.GitHubAuth = &atc.GitHubAuth{
			ClientID:      command.GitHubAuth.ClientID,
			ClientSecret:  command.GitHubAuth.ClientSecret,
			Organizations: command.GitHubAuth.Organizations,
			Users:         command.GitHubAuth.Users,
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

	_, _, _, err = target.Client().Team(command.TeamName).CreateOrUpdate(team)
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

	if command.UAAAuth.IsConfigured() {
		err := command.UAAAuth.Validate()
		if err != nil {
			return false, false, err
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
