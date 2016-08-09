package uaa

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/auth/verifier"
	"github.com/concourse/atc/db"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

const ProviderName = "uaa"

var Scopes = []string{"cloud_controller.read"}

type Provider interface {
	DisplayName() string
	PreTokenClient() *http.Client

	OAuthClient
	Verifier
}

type OAuthClient interface {
	AuthCodeURL(string, ...oauth2.AuthCodeOption) string
	Exchange(context.Context, string) (*oauth2.Token, error)
	Client(context.Context, *oauth2.Token) *http.Client
}

type Verifier interface {
	Verify(lager.Logger, *http.Client) (bool, error)
}

func NewProvider(
	uaaAuth *db.UAAAuth,
	redirectURL string,
) (Provider, error) {
	endpoint := oauth2.Endpoint{}
	if uaaAuth.AuthURL != "" && uaaAuth.TokenURL != "" {
		endpoint.AuthURL = uaaAuth.AuthURL
		endpoint.TokenURL = uaaAuth.TokenURL
	}

	transport := &http.Transport{
		DisableKeepAlives: true,
	}

	if uaaAuth.CFCACert != "" {
		caCertPool := x509.NewCertPool()
		ok := caCertPool.AppendCertsFromPEM([]byte(uaaAuth.CFCACert))
		if !ok {
			return uaaProvider{}, errors.New("failed to use cf certificate")
		}
		transport.TLSClientConfig = &tls.Config{
			RootCAs: caCertPool,
		}
	}

	disabledKeepAliveClient := &http.Client{
		Transport: transport,
	}

	return uaaProvider{
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
		PTClient: disabledKeepAliveClient,
	}, nil
}

type uaaProvider struct {
	*oauth2.Config
	// oauth2.Config implements the required Provider methods:
	// AuthCodeURL(string, ...oauth2.AuthCodeOption) string
	// Exchange(context.Context, string) (*oauth2.Token, error)
	// Client(context.Context, *oauth2.Token) *http.Client

	verifier.Verifier
	PTClient *http.Client
}

func (uaaProvider) DisplayName() string {
	return "UAA"
}

func (p uaaProvider) PreTokenClient() *http.Client {
	return p.PTClient
}
