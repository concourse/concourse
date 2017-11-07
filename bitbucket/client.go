package bitbucket

import "net/http"

//go:generate counterfeiter . Client

type Client interface {
	CurrentUser(*http.Client) (string, error)
	Teams(*http.Client, Role) ([]string, error)
}
