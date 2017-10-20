package bitbucketserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/SHyx0rmZ/go-bitbucket/bitbucket"
	"github.com/SHyx0rmZ/go-bitbucket/server"
	"github.com/concourse/atc/auth/verifier"
	"golang.org/x/net/context"
	"net/http"
)

type UserVerifier struct {
	users []string
}

func NewUserVerifier(users []string) verifier.Verifier {
	return UserVerifier{
		users: users,
	}
}

func (verifier UserVerifier) Verify(logger lager.Logger, c *http.Client) (bool, error) {
	bc, err := server.NewClient(context.WithValue(context.Background(), bitbucket.HTTPClient, c), "http://192.168.46.253:7990/")
	if err != nil {
		return false, err
	}

	currentUser, err := bc.CurrentUser()
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
