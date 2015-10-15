package github

import (
	"net/http"

	gogithub "github.com/google/go-github/github"
)

//go:generate counterfeiter . Client

type Client interface {
	Organizations(*http.Client) ([]string, error)
}

type client struct{}

func NewClient() Client {
	return &client{}
}

func (c *client) Organizations(httpClient *http.Client) ([]string, error) {
	client := gogithub.NewClient(httpClient)

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
