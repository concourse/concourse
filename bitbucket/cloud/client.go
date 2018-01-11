package cloud

import (
	api "github.com/SHyx0rmZ/go-bitbucket/cloud"
	"github.com/concourse/skymarshal/bitbucket"
	"net/http"
)

type client struct {
	endpoint string
}

func NewClient(endpoint string) bitbucket.Client {
	return &client{
		endpoint: endpoint,
	}
}

func (c *client) CurrentUser(httpClient *http.Client) (string, error) {
	bc, err := api.NewClient(httpClient, c.endpoint)
	if err != nil {
		return "", err
	}

	return bc.CurrentUser()
}

func (c *client) Repository(httpClient *http.Client, owner string, repository string) (bool, error) {
	bc, err := api.NewClient(httpClient, c.endpoint)
	if err != nil {
		return false, err
	}

	_, err = bc.Repository(owner + "/" + repository)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (c *client) Projects(httpClient *http.Client) ([]string, error) {
	return nil, nil
}

func (c *client) Teams(httpClient *http.Client, role string) ([]string, error) {
	bc, err := api.NewClient(httpClient, c.endpoint)
	if err != nil {
		return nil, err
	}

	ts, err := bc.TeamsWithRole(role)
	if err != nil {
		return nil, err
	}

	s := make([]string, len(ts))
	for i, t := range ts {
		s[i] = t.Username
	}

	return s, nil
}
