package provider

import (
	"net/http"

	"github.com/concourse/atc/db"

	"code.cloudfoundry.org/lager"

	"fmt"

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

type ProviderConstructor func(db.SavedTeam, string) (Provider, bool)

var providers map[string]ProviderConstructor

func init() {
	providers = make(map[string]ProviderConstructor)
}

func Register(providerName string, providerConstructor ProviderConstructor) error {
	if _, exists := providers[providerName]; exists {
		return fmt.Errorf("Provider already registered %s", providerName)
	}

	providers[providerName] = providerConstructor
	return nil
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

	newProvider, ok := provider(team, redirectURL)
	if !ok {
		return nil, false
	}

	return newProvider, ok
}
