package bitbucket

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/auth/verifier"
	"net/http"
)

type TeamVerifier struct {
	teams           []string
	role            Role
	bitbucketClient Client
}

func NewTeamVerifier(teams []string, role Role, bitbucketClient Client) verifier.Verifier {
	return TeamVerifier{
		teams:           teams,
		role:            role,
		bitbucketClient: bitbucketClient,
	}
}

func (verifier TeamVerifier) Verify(logger lager.Logger, httpClient *http.Client) (bool, error) {
	teams, err := verifier.bitbucketClient.Teams(httpClient, verifier.role)
	if err != nil {
		logger.Error("failed-to-get-teams", err)
		return false, err
	}

	for _, team := range teams {
		for _, verifierTeam := range verifier.teams {
			if team == verifierTeam {
				return true, nil
			}
		}
	}

	logger.Info("not-validated-teams", lager.Data{
		"have": teams,
		"want": verifier.teams,
	})

	return false, nil
}
