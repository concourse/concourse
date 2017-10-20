package cloud

import (
	api "github.com/SHyx0rmZ/go-bitbucket/cloud"
	"github.com/concourse/atc/auth/bitbucket"
	"net/http"
)

type client struct {
}

func NewClient() bitbucket.Client {
	return &client{}
}

func (c *client) CurrentUser(httpClient *http.Client) (string, error) {
	bc, err := api.NewClient(httpClient)
	if err != nil {
		return "", err
	}

	return bc.CurrentUser()
}
