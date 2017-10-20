package cloud

import (
	"github.com/concourse/atc/auth/verifier"
	"golang.org/x/oauth2"
	"net/http"
)

type Provider struct {
	*oauth2.Config
	verifier.Verifier
}

func (Provider) PreTokenClient() (*http.Client, error) {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			DisableKeepAlives: true,
		},
	}, nil
}
