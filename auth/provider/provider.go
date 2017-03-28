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

// If you can think of a better name, please change it
type TeamProvider interface {
	ProviderConstructor(db.SavedTeam, string) (Provider, bool)
	ProviderConfigured(db.Team) bool
}

var providers map[string]TeamProvider

func init() {
	providers = make(map[string]TeamProvider)
}

func Register(providerName string, providerConstructor TeamProvider) error {
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
	teamProvider, found := providers[providerName]
	if !found {
		return nil, false
	}

	newProvider, ok := teamProvider.ProviderConstructor(team, redirectURL)
	if !ok {
		return nil, false
	}

	return newProvider, ok
}

func GetProviders() map[string]TeamProvider {
	return providers
}
