package auth

import (
	"net/http"

	"golang.org/x/oauth2"
)

type Providers map[string]Provider

type Provider struct {
	*oauth2.Config
	Verifier
}

type Verifier interface {
	Verify(*http.Client) (bool, error)
}
