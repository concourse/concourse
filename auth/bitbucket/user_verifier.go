package bitbucket

import (
	"code.cloudfoundry.org/lager"
	api "github.com/SHyx0rmZ/go-bitbucket/bitbucket"
	"github.com/concourse/atc/auth/verifier"
	"net/http"
)

type UserVerifier struct {
	users  []string
	client api.Client
}

func NewUserVerifier(client api.Client, users []string) verifier.Verifier {
	return UserVerifier{
		users:  users,
		client: client,
	}
}

func (verifier UserVerifier) Verify(logger lager.Logger, c *http.Client) (bool, error) {
	verifier.client.SetHTTPClient(c)

	currentUser, err := verifier.client.CurrentUser()
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
