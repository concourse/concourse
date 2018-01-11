package cloud

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/skymarshal/bitbucket"
	"github.com/concourse/skymarshal/verifier"
	"net/http"
)

type TeamVerifier struct {
	teams           []string
	role            Role
	bitbucketClient bitbucket.Client
}

func NewTeamVerifier(teams []string, role Role, bitbucketClient bitbucket.Client) verifier.Verifier {
	return TeamVerifier{
		teams:           teams,
		role:            role,
		bitbucketClient: bitbucketClient,
	}
}

func (verifier TeamVerifier) Verify(logger lager.Logger, httpClient *http.Client) (bool, error) {
	if len(verifier.teams) == 0 {
		return false, nil
	}

	teams, err := verifier.bitbucketClient.Teams(httpClient, verifier.role.String())
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
