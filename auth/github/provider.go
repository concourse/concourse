package github

import (
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/verifier"
	"github.com/concourse/atc/db"
)

const ProviderName = "github"
const DisplayName = "GitHub"

var Scopes = []string{"read:org"}

type GitHubProvider struct {
	*oauth2.Config
	verifier.Verifier
}

func init() {
	provider.Register(ProviderName, NewGitHubProvider)
}

func NewGitHubProvider(
	team db.SavedTeam,
	redirectURL string,
) (provider.Provider, bool) {

	if team.GitHubAuth == nil {
		return nil, false
	}

	client := NewClient(team.GitHubAuth.APIURL)

	endpoint := github.Endpoint
	if team.GitHubAuth.AuthURL != "" && team.GitHubAuth.TokenURL != "" {
		endpoint.AuthURL = team.GitHubAuth.AuthURL
		endpoint.TokenURL = team.GitHubAuth.TokenURL
	}

	return GitHubProvider{
		Verifier: verifier.NewVerifierBasket(
			NewTeamVerifier(DBTeamsToGitHubTeams(team.GitHubAuth.Teams), client),
			NewOrganizationVerifier(team.GitHubAuth.Organizations, client),
			NewUserVerifier(team.GitHubAuth.Users, client),
		),
		Config: &oauth2.Config{
			ClientID:     team.GitHubAuth.ClientID,
			ClientSecret: team.GitHubAuth.ClientSecret,
			Endpoint:     endpoint,
			Scopes:       Scopes,
			RedirectURL:  redirectURL,
		},
	}, true
}

func DBTeamsToGitHubTeams(dbteams []db.GitHubTeam) []Team {
	teams := []Team{}
	for _, team := range dbteams {
		teams = append(teams, Team{
			Name:         team.TeamName,
			Organization: team.OrganizationName,
		})
	}
	return teams
}

func (GitHubProvider) PreTokenClient() (*http.Client, error) {
	return &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}, nil
}
