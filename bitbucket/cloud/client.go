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
	bc, err := api.NewClient(httpClient, "")
	if err != nil {
		return "", err
	}

	return bc.CurrentUser()
}

func (c *client) Teams(httpClient *http.Client, role bitbucket.Role) ([]string, error) {
	bc, err := api.NewClient(httpClient, "")
	if err != nil {
		return nil, err
	}

	ts, err := bc.TeamsWithRole(string(role))
	if err != nil {
		return nil, err
	}

	s := make([]string, len(ts))
	for i, t := range ts {
		s[i] = t.Username
	}

	return s, nil
}

func (c *client) Projects(httpClient *http.Client) ([]string, error) {
	return nil, nil
}

func (c *client) Repository(httpClient *http.Client, owner string, repository string) (bool, error) {
	bc, err := api.NewClient(httpClient, "")
	if err != nil {
		return false, err
	}

	_, err = bc.Repository(owner + "/" + repository)
	if err != nil {
		return false, err
	}

	return true, nil
}
