package github

import (
	"fmt"
	"net/http"
	"net/url"

	gogithub "github.com/google/go-github/github"
)

//go:generate counterfeiter . Client

type Client interface {
	CurrentUser(*http.Client) (string, error)
	Organizations(*http.Client) ([]string, error)
	Teams(*http.Client) (OrganizationTeams, error)
}

type client struct {
	baseURL string
}

func NewClient(baseURL string) Client {
	return &client{baseURL: baseURL}
}

type OrganizationTeams map[string][]string

func (c *client) CurrentUser(httpClient *http.Client) (string, error) {
	client, err := c.githubClient(httpClient)
	if err != nil {
		return "", err
	}

	currentUser, _, err := client.Users.Get("")
	if err != nil {
		return "", err
	}

	return *currentUser.Login, nil
}

func (c *client) Teams(httpClient *http.Client) (OrganizationTeams, error) {
	client, err := c.githubClient(httpClient)
	if err != nil {
		return nil, err
	}

	teams, _, err := client.Organizations.ListUserTeams(nil)
	if err != nil {
		return nil, err
	}

	organizationTeams := OrganizationTeams{}
	for _, team := range teams {
		organizationName := *team.Organization.Login

		if _, ok := organizationTeams[organizationName]; !ok {
			organizationTeams[organizationName] = []string{}
		}

		organizationTeams[organizationName] = append(organizationTeams[organizationName], *team.Name)
	}

	return organizationTeams, nil
}

func (c *client) Organizations(httpClient *http.Client) ([]string, error) {
	client, err := c.githubClient(httpClient)
	if err != nil {
		return nil, err
	}

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

func (c *client) githubClient(httpClient *http.Client) (*gogithub.Client, error) {
	client := gogithub.NewClient(httpClient)
	if c.baseURL != "" {
		var err error
		client.BaseURL, err = url.Parse(c.baseURL)
		if err != nil {
			return nil, fmt.Errorf("invalid github auth API URL '%s'", c.baseURL)
		}
	}

	return client, nil
}
