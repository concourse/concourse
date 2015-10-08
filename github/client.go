package github

import (
	gogithub "github.com/google/go-github/github"
	"github.com/pivotal-golang/lager"
	"golang.org/x/oauth2"
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
	client := newAPIClient(accessToken)

	orgs, _, err := client.Organizations.List("", nil)
	if err != nil {
		return nil, err
	}

	organizations := []string{}
	for _, org := range orgs {
		organizations = append(organizations, *org.Login)
	}
	return organizations, nil
}

func newAPIClient(accessToken string) *gogithub.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)

	tc := oauth2.NewClient(oauth2.NoContext, ts)
	return gogithub.NewClient(tc)
}
