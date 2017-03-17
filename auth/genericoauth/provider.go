package genericoauth

import (
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/auth/verifier"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

const ProviderName = "oauth"

type Provider struct {
	verifier.Verifier
	Config ConfigOverride
}

type ConfigOverride struct {
	oauth2.Config
	AuthURLParams map[string]string
}

type NoopVerifier struct{}

func (v NoopVerifier) Verify(logger lager.Logger, client *http.Client) (bool, error) {
	return true, nil
}

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
