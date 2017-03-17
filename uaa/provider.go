package uaa

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"

	"github.com/concourse/atc/auth/verifier"
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
