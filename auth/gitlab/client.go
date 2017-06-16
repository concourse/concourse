package gitlab

import (
	"fmt"
	"net/http"
	"net/url"

	gogitlab "github.com/xanzy/go-gitlab"
)

//go:generate counterfeiter . Client

type Client interface {
	Groups(*http.Client) ([]string, error)
}

type client struct {
	baseURL string
}

func NewClient(baseURL string) Client {
	return &client{baseURL: baseURL}
}

func (c *client) gitlabClient(httpClient *http.Client) (*gogitlab.Client, error) {
	client := gogitlab.NewClient(httpClient, "token")
	if c.baseURL != "" {
		var err error
		gitlabBaseURL, err := url.Parse(c.baseURL)
		if err != nil {
			return nil, fmt.Errorf("invalid gitlab auth API URL '%s'", c.baseURL)
		}
		client.SetBaseURL(gitlabBaseURL.String())
	}
	return client, nil
}

func (c *client) Groups(httpClient *http.Client) ([]string, error) {
	client, err := c.gitlabClient(httpClient)
	if err != nil {
		return nil, err
	}

	nextPage := 1
	usersGroups := []string{}

	for nextPage != 0 {
		opts := &gogitlab.ListGroupsOptions{gogitlab.ListOptions{Page: nextPage}, nil, nil}
		groups, resp, err := client.Groups.ListGroups(opts)
		if err != nil {
			return nil, err
		}

		for _, g := range groups {
			usersGroups = append(usersGroups, g.Name)
		}

		nextPage = resp.NextPage
	}

	return usersGroups, nil
}
