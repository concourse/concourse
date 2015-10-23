package github

import (
	"net/http"

	"github.com/concourse/atc/auth"
)

type Team struct {
	Name         string
	Organization string
}

type TeamVerifier struct {
	teams        []Team
	gitHubClient Client
}

func NewTeamVerifier(
	teams []Team,
	gitHubClient Client,
) auth.Verifier {
	return &TeamVerifier{
		teams:        teams,
		gitHubClient: gitHubClient,
	}
}

func (verifier *TeamVerifier) Verify(httpClient *http.Client) (bool, error) {
	usersOrgTeams, err := verifier.gitHubClient.Teams(httpClient)
	if err != nil {
		return false, err
	}

	for _, team := range verifier.teams {
		if teams, ok := usersOrgTeams[team.Organization]; ok {
			for _, teamUserBelongsTo := range teams {
				if teamUserBelongsTo == team.Name {
					return true, nil
				}
			}
		}
	}

	return false, nil
}
