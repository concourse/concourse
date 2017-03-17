package provider

import (
	"net/http"

	"github.com/concourse/atc/auth/genericoauth"
	"github.com/concourse/atc/auth/github"
	"github.com/concourse/atc/auth/uaa"
	"github.com/concourse/atc/auth/verifier"
	"github.com/concourse/atc/db"

	"code.cloudfoundry.org/lager"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

//go:generate counterfeiter . Provider

type Provider interface {
	PreTokenClient() (*http.Client, error)

	OAuthClient
	Verifier
}

type OAuthClient interface {
	AuthCodeURL(string, ...oauth2.AuthCodeOption) string
	Exchange(context.Context, string) (*oauth2.Token, error)
	Client(context.Context, *oauth2.Token) *http.Client
}

//go:generate counterfeiter . Verifier

type Verifier interface {
	Verify(lager.Logger, *http.Client) (bool, error)
}

type ProviderConstructor func() Provider {

}

var providers map[string]ProviderConstructor

func Register(providerName string, providerConstructor ProviderConstructor) {
	providers = append(providers, providerConstructor)
}

func NewProvider(
	team db.SavedTeam,
	providerName string,
	redirectURL string,
) (Provider, bool) {

	provider, found := providers[providerName]
	if !found {
		return nil, false
	}

	return provider(team, redirectURL), true

	// switch providerName {
	//
	// case "github":
	// 	if team.GitHubAuth == nil {
	// 		return nil, false
	// 	}
	//
	// 	return NewGitHubProvider(team.GitHubAuth, redirectURL), true
	//
	// case "uaa":
	// 	if team.UAAAuth == nil {
	// 		return nil, false
	// 	}
	//
	// 	return NewUAAProvider(team.UAAAuth, redirectURL), true
	//
	// case "oauth":
	// 	if team.GenericOAuth == nil {
	// 		return nil, false
	// 	}
	//
	// 	return NewGenericProvider(team.GenericOAuth, redirectURL), true
	//
	// default:
	// 	return nil, false
	// }

}

func NewGitHubProvider(
	team db.SavedTeam,
	redirectURL string,
) Provider {

	client := github.NewClient(team.GitHubAuth.APIURL)

	endpoint := oauth2.Endpoint{}
	if team.GitHubAuth.AuthURL != "" && team.GitHubAuth.TokenURL != "" {
		endpoint.AuthURL = team.GitHubAuth.AuthURL
		endpoint.TokenURL = team.GitHubAuth.TokenURL
	}

	return github.GitHubProvider{
		Verifier: verifier.NewVerifierBasket(
			github.NewTeamVerifier(github.DBTeamsToGitHubTeams(team.GitHubAuth.Teams), client),
			github.NewOrganizationVerifier(team.GitHubAuth.Organizations, client),
			github.NewUserVerifier(team.GitHubAuth.Users, client),
		),
		Config: &oauth2.Config{
			ClientID:     team.GitHubAuth.ClientID,
			ClientSecret: team.GitHubAuth.ClientSecret,
			Endpoint:     endpoint,
			Scopes:       github.Scopes,
			RedirectURL:  redirectURL,
		},
	}
}

func NewUAAProvider(
	team db.SavedTeam,
	redirectURL string,
) Provider {

	endpoint := oauth2.Endpoint{}
	if team.UAAAuth.AuthURL != "" && team.UAAAuth.TokenURL != "" {
		endpoint.AuthURL = team.UAAAuth.AuthURL
		endpoint.TokenURL = team.UAAAuth.TokenURL
	}

	return uaa.UAAProvider{
		Verifier: uaa.NewSpaceVerifier(
			team.UAAAuth.CFSpaces,
			team.UAAAuth.CFURL,
		),
		Config: &oauth2.Config{
			ClientID:     team.UAAAuth.ClientID,
			ClientSecret: team.UAAAuth.ClientSecret,
			Endpoint:     endpoint,
			Scopes:       uaa.Scopes,
			RedirectURL:  redirectURL,
		},
		CFCACert: team.UAAAuth.CFCACert,
	}
}

func NewGenericProvider(
	team db.SavedTeam,
	redirectURL string,
) Provider {

	endpoint := oauth2.Endpoint{}
	if team.GenericOAuth.AuthURL != "" && team.GenericOAuth.TokenURL != "" {
		endpoint.AuthURL = team.GenericOAuth.AuthURL
		endpoint.TokenURL = team.GenericOAuth.TokenURL
	}

	var oauthVerifier verifier.Verifier
	if team.GenericOAuth.Scope != "" {
		oauthVerifier = genericoauth.NewScopeVerifier(team.GenericOAuth.Scope)
	} else {
		oauthVerifier = genericoauth.NoopVerifier{}
	}

	return genericoauth.Provider{
		Verifier: oauthVerifier,
		Config: genericoauth.ConfigOverride{
			Config: oauth2.Config{
				ClientID:     team.GenericOAuth.ClientID,
				ClientSecret: team.GenericOAuth.ClientSecret,
				Endpoint:     endpoint,
				RedirectURL:  redirectURL,
			},
			AuthURLParams: team.GenericOAuth.AuthURLParams,
		},
	}
}
