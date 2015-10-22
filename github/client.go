package github

import (
	"net/http"

	gogithub "github.com/google/go-github/github"
)

//go:generate counterfeiter . Client

type Client interface {
	Teams(*http.Client) (OrganizationTeams, error)
}

type client struct{}

func NewClient() Client {
	return &client{}
}

type OrganizationTeams map[string][]string

func (c *client) Teams(httpClient *http.Client) (OrganizationTeams, error) {
	client := gogithub.NewClient(httpClient)
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

		organizationTeams[organizationName] = append(organizationTeams[organizationName], *team.Slug)
	}

	return organizationTeams, nil
}
