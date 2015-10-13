package auth

import (
	"github.com/concourse/atc/web/routes"
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/github"
	"github.com/tedsuo/rata"
)

var githubScopes = []string{"read:org"}

func RegisterGithub(clientID string, clientSecret string, externalURL string) error {
	route, err := routes.Routes.CreatePathForRoute(routes.OAuthCallback, rata.Params{
		"provider": "github",
	})
	if err != nil {
		return err
	}

	goth.UseProviders(
		github.New(
			clientID,
			clientSecret,
			externalURL+route,
			githubScopes...,
		),
	)

	return nil
}
