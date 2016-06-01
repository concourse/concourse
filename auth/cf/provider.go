package cf

import (
	"net/http"

	"github.com/concourse/atc/auth/verifier"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
	"golang.org/x/oauth2"
)

const ProviderName = "cf"

var Scopes = []string{"cloud_controller.read"}

type NoopVerifier struct{}

func (v NoopVerifier) Verify(logger lager.Logger, client *http.Client) (bool, error) {
	return true, nil
}

func NewProvider(
	cfAuth *db.CFAuth,
	redirectURL string,
) Provider {
	endpoint := oauth2.Endpoint{}
	if cfAuth.AuthURL != "" && cfAuth.TokenURL != "" {
		endpoint.AuthURL = cfAuth.AuthURL
		endpoint.TokenURL = cfAuth.TokenURL
	}

	return Provider{
		Verifier: NoopVerifier{},
		Config: &oauth2.Config{
			ClientID:     cfAuth.ClientID,
			ClientSecret: cfAuth.ClientSecret,
			Endpoint:     endpoint,
			Scopes:       Scopes,
			RedirectURL:  redirectURL,
		},
	}
}

type Provider struct {
	*oauth2.Config
	// oauth2.Config implements the required Provider methods:
	// AuthCodeURL(string, ...oauth2.AuthCodeOption) string
	// Exchange(context.Context, string) (*oauth2.Token, error)
	// Client(context.Context, *oauth2.Token) *http.Client

	verifier.Verifier
}

func (Provider) DisplayName() string {
	return "CF"
}
