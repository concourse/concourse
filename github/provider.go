package github

import (
	"github.com/concourse/atc/auth/provider"
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
) provider.Provider {
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

	return Provider{
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

type Provider struct {
	*oauth2.Config
	provider.Verifier
}

func (Provider) DisplayName() string {
	return "GitHub"
}
