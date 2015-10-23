package github

import (
	"net/http"
	"strings"

	"github.com/concourse/atc/auth"
)

type TeamVerifier struct {
	teams        []string
	gitHubClient Client
}

func NewTeamVerifier(
	teams []string,
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
		verifierOrgTeam := strings.Split(team, "/")

		if _, ok := usersOrgTeams[verifierOrgTeam[0]]; ok {
			for _, teamUserBelongsTo := range usersOrgTeams[verifierOrgTeam[0]] {
				if teamUserBelongsTo == verifierOrgTeam[1] {
					return true, nil
				}
			}
		}
	}

	return false, nil
}
