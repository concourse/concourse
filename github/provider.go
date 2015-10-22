package github

import (
	"github.com/concourse/atc/auth"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

const ProviderName = "github"

var Scopes = []string{"read:org"}

func NewProvider(
	teams []string,
	clientID string,
	clientSecret string,
	redirectURL string,
) auth.Provider {
	return provider{
		Verifier: NewTeamVerifier(teams, NewClient()),
		Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     github.Endpoint,
			Scopes:       Scopes,
			RedirectURL:  redirectURL,
		},
	}
}

type provider struct {
	*oauth2.Config
	auth.Verifier
}

func (provider) DisplayName() string {
	return "GitHub"
}
