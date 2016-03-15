package github

import (
	"net/http"
	"net/url"

	gogithub "github.com/google/go-github/github"
)

//go:generate counterfeiter . Client

type Client interface {
	CurrentUser(*http.Client, string) (string, error)
	Organizations(*http.Client, string) ([]string, error)
	Teams(*http.Client, string) (OrganizationTeams, error)
}

type client struct{}

func NewClient() Client {
	return &client{}
}

type OrganizationTeams map[string][]string

func (c *client) CurrentUser(httpClient *http.Client, APIURL string) (string, error) {
	client := gogithub.NewClient(httpClient)
	if APIURL != "" {
		client.BaseURL, _ = url.Parse(APIURL)
	}

	currentUser, _, err := client.Users.Get("")
	if err != nil {
		return "", err
	}

	return *currentUser.Login, nil
}

func (c *client) Teams(httpClient *http.Client, APIURL string) (OrganizationTeams, error) {
	client := gogithub.NewClient(httpClient)
	if APIURL != "" {
		client.BaseURL, _ = url.Parse(APIURL)
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

func (c *client) Organizations(httpClient *http.Client, APIURL string) ([]string, error) {
	client := gogithub.NewClient(httpClient)
	if APIURL != "" {
		client.BaseURL, _ = url.Parse(APIURL)
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
