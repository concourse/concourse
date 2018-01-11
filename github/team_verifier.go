package github

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/skymarshal/verifier"
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
) verifier.Verifier {
	return TeamVerifier{
		teams:        teams,
		gitHubClient: gitHubClient,
	}
}

func (verifier TeamVerifier) Verify(logger lager.Logger, httpClient *http.Client) (bool, error) {
	usersOrgTeams, err := verifier.gitHubClient.Teams(httpClient)
	if err != nil {
		logger.Error("failed-to-get-teams", err)
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

	logger.Info("not-in-teams", lager.Data{
		"have": usersOrgTeams,
		"want": verifier.teams,
	})

	return false, nil
}
