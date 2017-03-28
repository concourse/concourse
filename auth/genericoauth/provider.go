package genericoauth

import (
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/verifier"
	"github.com/concourse/atc/db"
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

func init() {
	provider.Register(ProviderName, GenericTeamProvider{})
}

type GenericTeamProvider struct{}

func (GenericTeamProvider) ProviderConfigured(team db.Team) bool {
	return team.GenericOAuth != nil
}

func (GenericTeamProvider) ProviderConstructor(
	team db.SavedTeam,
	redirectURL string,
) (provider.Provider, bool) {

	if team.GenericOAuth == nil {
		return nil, false
	}

	endpoint := oauth2.Endpoint{}
	if team.GenericOAuth.AuthURL != "" && team.GenericOAuth.TokenURL != "" {
		endpoint.AuthURL = team.GenericOAuth.AuthURL
		endpoint.TokenURL = team.GenericOAuth.TokenURL
	}

	var oauthVerifier verifier.Verifier
	if team.GenericOAuth.Scope != "" {
		oauthVerifier = NewScopeVerifier(team.GenericOAuth.Scope)
	} else {
		oauthVerifier = NoopVerifier{}
	}

	return Provider{
		Verifier: oauthVerifier,
		Config: ConfigOverride{
			Config: oauth2.Config{
				ClientID:     team.GenericOAuth.ClientID,
				ClientSecret: team.GenericOAuth.ClientSecret,
				Endpoint:     endpoint,
				RedirectURL:  redirectURL,
			},
			AuthURLParams: team.GenericOAuth.AuthURLParams,
		},
	}, true
}

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
