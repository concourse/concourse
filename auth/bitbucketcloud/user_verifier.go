package bitbucketcloud

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/auth/verifier"
	"github.com/SHyx0rmZ/go-bitbucket/cloud"
)

type UserVerifier struct {
	users        []string
}

func NewUserVerifier(
	users []string,
) verifier.Verifier {
	return UserVerifier{
		users:        users,
	}
}

func (verifier UserVerifier) Verify(logger lager.Logger, httpClient *http.Client) (bool, error) {
	bitbucketClient, err := cloud.NewClient(httpClient)
	if err != nil {
		return false, err
	}
	accessibleUsers, err := bitbucketClient.Users()
	if err != nil || len(accessibleUsers) != 1 {
		logger.Error("failed-to-get-current-user", err)
		return false, err
	}

	currentUser := accessibleUsers[0].GetName()

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
