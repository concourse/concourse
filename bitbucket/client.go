package bitbucket

import "net/http"

//go:generate counterfeiter . Client

type Client interface {
	CurrentUser(*http.Client) (string, error)
	Projects(*http.Client) ([]string, error)
	Repository(client *http.Client, owner string, repository string) (bool, error)
	Teams(*http.Client, string) ([]string, error)
}
