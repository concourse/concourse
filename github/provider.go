package github

import (
	"net/http"

	"golang.org/x/oauth2"

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
