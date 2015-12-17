package github

import (
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/db"
	"github.com/tedsuo/rata"
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

func NewGitHubProvider(
	gitHubAuth db.GitHubAuth,
) (provider.Provider, error) {
	redirectURL, err := auth.OAuthRoutes.CreatePathForRoute(auth.OAuthCallback, rata.Params{
		"provider": ProviderName,
	})
	if err != nil {
		return nil, err
	}

	return NewProvider(gitHubAuth, redirectURL), nil
}

func NewProvider(
	gitHubAuth db.GitHubAuth,
	redirectURL string,
) provider.Provider {
	client := NewClient()

	return Provider{
		Verifier: NewVerifierBasket(
			NewTeamVerifier(dbTeamsToGitHubTeams(gitHubAuth.Teams), client),
			NewOrganizationVerifier(gitHubAuth.Organizations, client),
			NewUserVerifier(gitHubAuth.Users, client),
		),
		Config: &oauth2.Config{
			ClientID:     gitHubAuth.ClientID,
			ClientSecret: gitHubAuth.ClientSecret,
			Endpoint:     github.Endpoint,
			Scopes:       Scopes,
			RedirectURL:  redirectURL,
		},
	}
}

type Provider struct {
	*oauth2.Config
	provider.Verifier
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
