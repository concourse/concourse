package uaa

import (
	"net/http"

	"github.com/concourse/atc/auth/verifier"
	"github.com/concourse/atc/db"
	"code.cloudfoundry.org/lager"
	"golang.org/x/oauth2"
)

const ProviderName = "uaa"

var Scopes = []string{"cloud_controller.read"}

type NoopVerifier struct{}

func (v NoopVerifier) Verify(logger lager.Logger, client *http.Client) (bool, error) {
	return true, nil
}

func NewProvider(
	uaaAuth *db.UAAAuth,
	redirectURL string,
) Provider {
	endpoint := oauth2.Endpoint{}
	if uaaAuth.AuthURL != "" && uaaAuth.TokenURL != "" {
		endpoint.AuthURL = uaaAuth.AuthURL
		endpoint.TokenURL = uaaAuth.TokenURL
	}

	return Provider{
		Verifier: SpaceVerifier{
			spaceGUIDs: uaaAuth.CFSpaces,
			cfAPIURL:   uaaAuth.CFURL,
		},
		Config: &oauth2.Config{
			ClientID:     uaaAuth.ClientID,
			ClientSecret: uaaAuth.ClientSecret,
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
	return "UAA"
}
