package uaa

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"

	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/verifier"
	"github.com/concourse/atc/db"
	"golang.org/x/oauth2"
)

const ProviderName = "uaa"
const DisplayName = "UAA"

var Scopes = []string{"cloud_controller.read"}

type UAAProvider struct {
	*oauth2.Config
	verifier.Verifier
	CFCACert string
}

func init() {
	provider.Register(ProviderName, UAATeamProvider{})
}

type UAATeamProvider struct{}

func (UAATeamProvider) ProviderConfigured(team db.Team) bool {
	return team.UAAAuth != nil
}

func (UAATeamProvider) ProviderConstructor(
	team db.SavedTeam,
	redirectURL string,
) (provider.Provider, bool) {

	if team.UAAAuth == nil {
		return nil, false
	}

	endpoint := oauth2.Endpoint{}
	if team.UAAAuth.AuthURL != "" && team.UAAAuth.TokenURL != "" {
		endpoint.AuthURL = team.UAAAuth.AuthURL
		endpoint.TokenURL = team.UAAAuth.TokenURL
	}

	return UAAProvider{
		Verifier: NewSpaceVerifier(
			team.UAAAuth.CFSpaces,
			team.UAAAuth.CFURL,
		),
		Config: &oauth2.Config{
			ClientID:     team.UAAAuth.ClientID,
			ClientSecret: team.UAAAuth.ClientSecret,
			Endpoint:     endpoint,
			Scopes:       Scopes,
			RedirectURL:  redirectURL,
		},
		CFCACert: team.UAAAuth.CFCACert,
	}, true
}

func (p UAAProvider) PreTokenClient() (*http.Client, error) {
	transport := &http.Transport{
		DisableKeepAlives: true,
	}

	if p.CFCACert != "" {
		caCertPool := x509.NewCertPool()
		ok := caCertPool.AppendCertsFromPEM([]byte(p.CFCACert))
		if !ok {
			return nil, errors.New("failed to use cf certificate")
		}

		transport.TLSClientConfig = &tls.Config{
			RootCAs: caCertPool,
		}
	}

	return &http.Client{
		Transport: transport,
	}, nil
}
