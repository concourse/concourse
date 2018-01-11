package bitbucket

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/skymarshal/verifier"
	"net/http"
)

type UserVerifier struct {
	users           []string
	bitbucketClient Client
}

func NewUserVerifier(users []string, bitbucketClient Client) verifier.Verifier {
	return UserVerifier{
		users:           users,
		bitbucketClient: bitbucketClient,
	}
}

func (verifier UserVerifier) Verify(logger lager.Logger, httpClient *http.Client) (bool, error) {
	if len(verifier.users) == 0 {
		return false, nil
	}

	currentUser, err := verifier.bitbucketClient.CurrentUser(httpClient)
	if err != nil {
		logger.Error("failed-to-get-current-user", err)
		return false, err
	}

	for _, user := range verifier.users {
		if user == currentUser {
			return true, nil
		}
	}

	logger.Info("not-validated-user", lager.Data{
		"have": currentUser,
		"want": verifier.users,
	})

	return false, nil
}
