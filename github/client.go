package github

import (
	"net/http"

	"github.com/octokit/go-octokit/octokit"
	"github.com/pivotal-golang/lager"
)

const (
	APIURL       = "https://api.github.com/"
	APIUserAgent = "concourse-api"
)

//go:generate counterfeiter . Client

type Client interface {
	GetOrganizations(accessToken string) ([]string, error)
}

type client struct {
	logger lager.Logger
}

func NewClient(logger lager.Logger) Client {
	return &client{
		logger: logger,
	}
}

func (c *client) GetOrganizations(accessToken string) ([]string, error) {
	octoClient := newOctoClient(accessToken)

	orgs, result := octoClient.Organization().YourOrganizations(&octokit.YourOrganizationsURL, octokit.M{})
	if result.Err != nil {
		c.logger.Error("failed-to-get-github-organizations", result.Err)
		return nil, result.Err
	}

	organizations := []string{}
	for _, org := range orgs {
		organizations = append(organizations, org.Login)
	}
	return organizations, nil
}

func newOctoClient(accessToken string) *octokit.Client {
	return octokit.NewClientWith(
		APIURL,
		APIUserAgent,
		octokit.TokenAuth{
			AccessToken: accessToken,
		},
		&http.Client{
			Transport: &http.Transport{},
		},
	)
}
