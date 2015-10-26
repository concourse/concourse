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

	User string
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
	var users []string

	for _, method := range methods {
		if method.Organization != "" && method.Team != "" {
			teams = append(teams, Team{
				Name:         method.Team,
				Organization: method.Organization,
			})
		} else if method.Organization != "" {
			orgs = append(orgs, method.Organization)
		} else if method.User != "" {
			users = append(users, method.User)
		}
	}

	return provider{
		Verifier: NewVerifierBasket(
			NewTeamVerifier(teams, client),
			NewOrganizationVerifier(orgs, client),
			NewUserVerifier(users, client),
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
