package bitbucketcloud

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/SHyx0rmZ/go-bitbucket/cloud"
	"github.com/concourse/atc/auth/verifier"
)

type UserVerifier struct {
	users []string
}

func NewUserVerifier(
	users []string,
) verifier.Verifier {
	return UserVerifier{
		users: users,
	}
}

func (verifier UserVerifier) Verify(logger lager.Logger, httpClient *http.Client) (bool, error) {
	bitbucketClient, err := cloud.NewClient(httpClient)
	if err != nil {
		return false, err
	}

	currentUser, err := bitbucketClient.CurrentUser()
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
