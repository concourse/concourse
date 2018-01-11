package provider

import (
	"net/http"

	flags "github.com/jessevdk/go-flags"

	"code.cloudfoundry.org/lager"

	"encoding/json"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

type AuthType string

const (
	AuthTypeBasic AuthType = "basic"
	AuthTypeOAuth AuthType = "oauth"
)

type AuthMethod struct {
	Type AuthType `json:"type"`

	DisplayName string `json:"display_name"`
	AuthURL     string `json:"auth_url"`
}

type AuthToken struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// ==================================
// ==================================
// ==================================
// ==================================
// ==================================

//go:generate counterfeiter . Provider

type Provider interface {
	PreTokenClient() (*http.Client, error)

	OAuthClient
	Verifier
}

type OAuthClient interface {
	AuthCodeURL(string, ...oauth2.AuthCodeOption) (string, error)
	Exchange(context.Context, *http.Request) (*oauth2.Token, error)
	Client(context.Context, *oauth2.Token) *http.Client
}

//go:generate counterfeiter . Verifier

type Verifier interface {
	Verify(lager.Logger, *http.Client) (bool, error)
}

//go:generate counterfeiter . AuthConfig

type AuthConfig interface {
	IsConfigured() bool
	Validate() error
	AuthMethod(oauthBaseURL string, teamName string) AuthMethod
}

type AuthConfigs map[string]AuthConfig

//go:generate counterfeiter . TeamProvider

type TeamProvider interface { // XXX rename to ProviderFactory
	ProviderConstructor(AuthConfig, string) (Provider, bool)
	AddAuthGroup(*flags.Group) AuthConfig
	UnmarshalConfig(*json.RawMessage) (AuthConfig, error)
}

var providers map[string]TeamProvider

func init() {
	providers = make(map[string]TeamProvider)
}

func Register(providerName string, providerConstructor TeamProvider) {
	providers[providerName] = providerConstructor
}

func NewProvider(
	auth *json.RawMessage,
	providerName string,
	redirectURL string,
) (Provider, bool) {
	teamProvider, found := providers[providerName]
	if !found {
		return nil, false
	}

	authConfig, err := teamProvider.UnmarshalConfig(auth)
	if err != nil {
		return nil, false
	}

	newProvider, ok := teamProvider.ProviderConstructor(authConfig, redirectURL)
	if !ok {
		return nil, false
	}

	return newProvider, ok
}

func GetProviders() map[string]TeamProvider {
	return providers
}
