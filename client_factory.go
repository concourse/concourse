package web

import (
	"crypto/tls"
	"net/http"

	"github.com/concourse/go-concourse/concourse"
)

//go:generate counterfeiter . ClientFactory

type ClientFactory interface {
	Build(request *http.Request) concourse.Client
}

type clientFactory struct {
	apiEndpoint                 string
	allowSelfSignedCertificates bool
}

func NewClientFactory(apiEndpoint string, allowSelfSignedCertificates bool) ClientFactory {
	return &clientFactory{
		apiEndpoint:                 apiEndpoint,
		allowSelfSignedCertificates: allowSelfSignedCertificates,
	}
}

func (cf *clientFactory) Build(r *http.Request) concourse.Client {
	transport := authorizationTransport{
		Authorization: r.Header.Get("Authorization"),

		Base: &http.Transport{

			DisableKeepAlives: true, // disable connection pooling
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cf.allowSelfSignedCertificates,
			},
		},
	}

	httpClient := &http.Client{
		Transport: transport,
	}

	return concourse.NewClient(cf.apiEndpoint, httpClient)
}

type authorizationTransport struct {
	Base          http.RoundTripper
	Authorization string
}

func (transport authorizationTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Authorization", transport.Authorization)
	return transport.Base.RoundTrip(r)
}
