package github

import (
	"github.com/concourse/atc/auth"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

const ProviderName = "github"

var Scopes = []string{"read:org"}

type AuthorizationMethod struct {
	Organization string
	Team         string
}

func NewProvider(
	methods []AuthorizationMethod,
	clientID string,
	clientSecret string,
	redirectURL string,
) auth.Provider {
	client := NewClient()

	var teams []Team
	var orgs []string

	for _, method := range methods {
		if method.Organization != "" && method.Team != "" {
			teams = append(teams, Team{
				Name:         method.Team,
				Organization: method.Organization,
			})
		} else {
			orgs = append(orgs, method.Organization)
		}
	}

	return provider{
		Verifier: NewVerifierBasket(
			NewTeamVerifier(teams, client),
			NewOrganizationVerifier(orgs, client),
		),
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
