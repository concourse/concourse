package genericoauth

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/auth/verifier"
	"github.com/concourse/atc/db"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

const ProviderName = "oauth"

type NoopVerifier struct{}

func (v NoopVerifier) Verify(logger lager.Logger, client *http.Client) (bool, error) {
	return true, nil
}

func NewProvider(
	genericOAuth *db.GenericOAuth,
	redirectURL string,
) Provider {
	endpoint := oauth2.Endpoint{}
	if genericOAuth.AuthURL != "" && genericOAuth.TokenURL != "" {
		endpoint.AuthURL = genericOAuth.AuthURL
		endpoint.TokenURL = genericOAuth.TokenURL
	}

	return Provider{
		Verifier: NoopVerifier{},
		Config: ConfigOverride{
			Config: oauth2.Config{
				ClientID:     genericOAuth.ClientID,
				ClientSecret: genericOAuth.ClientSecret,
				Endpoint:     endpoint,
				RedirectURL:  redirectURL,
			},
			AuthURLParams: genericOAuth.AuthURLParams,
		},
	}
}

type Provider struct {
	verifier.Verifier
	Config ConfigOverride
}

type ConfigOverride struct {
	oauth2.Config
	AuthURLParams map[string]string
}

// oauth2.Config implements the required Provider methods:
// AuthCodeURL(string, ...oauth2.AuthCodeOption) string
// Exchange(context.Context, string) (*oauth2.Token, error)
// Client(context.Context, *oauth2.Token) *http.Client

// override the default Provider method implementation from
// oauth2.Config in order to pass the extra configured Auth
// URL parameters

func (provider Provider) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	for key, value := range provider.Config.AuthURLParams {
		opts = append(opts, oauth2.SetAuthURLParam(key, value))
	}
	return provider.Config.AuthCodeURL(state, opts...)
}

func (provider Provider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return provider.Config.Exchange(ctx, code)
}

func (provider Provider) Client(ctx context.Context, t *oauth2.Token) *http.Client {
	return provider.Config.Client(ctx, t)
}

func (Provider) PreTokenClient() (*http.Client, error) {
	return &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}, nil
}
