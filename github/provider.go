package github

import (
	"github.com/concourse/atc/db"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

const ProviderName = "github"

var Scopes = []string{"read:org"}

type AuthorizationMethod struct {
	Organization string
	Team         string

	User string
}

func NewProvider(
	gitHubAuth db.GitHubAuth,
	redirectURL string,
) Provider {
	client := NewClient()

	endpoint := github.Endpoint
	if gitHubAuth.AuthURL != "" && gitHubAuth.TokenURL != "" {
		endpoint.AuthURL = gitHubAuth.AuthURL
		endpoint.TokenURL = gitHubAuth.TokenURL
	}

	return Provider{
		Verifier: NewVerifierBasket(
			NewTeamVerifier(dbTeamsToGitHubTeams(gitHubAuth.Teams), client, gitHubAuth.APIURL),
			NewOrganizationVerifier(gitHubAuth.Organizations, client, gitHubAuth.APIURL),
			NewUserVerifier(gitHubAuth.Users, client, gitHubAuth.APIURL),
		),
		Config: &oauth2.Config{
			ClientID:     gitHubAuth.ClientID,
			ClientSecret: gitHubAuth.ClientSecret,
			Endpoint:     endpoint,
			Scopes:       Scopes,
			RedirectURL:  redirectURL,
		},
	}
}

type Provider struct {
	*oauth2.Config
	// oauth2.Config implements the required Provider methods:
	// AuthCodeURL(string, ...oauth2.AuthCodeOption) string
	// Exchange(context.Context, string) (*oauth2.Token, error)
	// Client(context.Context, *oauth2.Token) *http.Client

	Verifier
}

func dbTeamsToGitHubTeams(dbteams []db.GitHubTeam) []Team {
	teams := []Team{}
	for _, team := range dbteams {
		teams = append(teams, Team{
			Name:         team.TeamName,
			Organization: team.OrganizationName,
		})
	}
	return teams
}

func (Provider) DisplayName() string {
	return "GitHub"
}
