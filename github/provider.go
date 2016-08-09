package github

import (
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/auth/verifier"
	"github.com/concourse/atc/db"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

const ProviderName = "github"

var Scopes = []string{"read:org"}

type Provider interface {
	DisplayName() string
	PreTokenClient() *http.Client

	OAuthClient
	Verifier
}

type OAuthClient interface {
	AuthCodeURL(string, ...oauth2.AuthCodeOption) string
	Exchange(context.Context, string) (*oauth2.Token, error)
	Client(context.Context, *oauth2.Token) *http.Client
}

type Verifier interface {
	Verify(lager.Logger, *http.Client) (bool, error)
}

func NewProvider(
	gitHubAuth *db.GitHubAuth,
	redirectURL string,
) Provider {
	client := NewClient(gitHubAuth.APIURL)

	endpoint := github.Endpoint
	if gitHubAuth.AuthURL != "" && gitHubAuth.TokenURL != "" {
		endpoint.AuthURL = gitHubAuth.AuthURL
		endpoint.TokenURL = gitHubAuth.TokenURL
	}

	return gitHubProvider{
		Verifier: verifier.NewVerifierBasket(
			NewTeamVerifier(dbTeamsToGitHubTeams(gitHubAuth.Teams), client),
			NewOrganizationVerifier(gitHubAuth.Organizations, client),
			NewUserVerifier(gitHubAuth.Users, client),
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

type gitHubProvider struct {
	*oauth2.Config
	// oauth2.Config implements the required Provider methods:
	// AuthCodeURL(string, ...oauth2.AuthCodeOption) string
	// Exchange(context.Context, string) (*oauth2.Token, error)
	// Client(context.Context, *oauth2.Token) *http.Client

	verifier.Verifier
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

func (gitHubProvider) DisplayName() string {
	return "GitHub"
}

func (gitHubProvider) PreTokenClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
}
