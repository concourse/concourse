package bitbucketcloud

import (
	"github.com/concourse/atc/auth/verifier"
	"golang.org/x/oauth2"
	"net/http"
)

type BitbucketCloudProvider struct {
	*oauth2.Config
	verifier.Verifier
}

func (BitbucketCloudProvider) PreTokenClient() (*http.Client, error) {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			DisableKeepAlives: true,
		},
	}, nil
}
