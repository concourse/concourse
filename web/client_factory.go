package web

import (
	"net/http"

	"github.com/concourse/go-concourse/concourse"
)

type ClientFactory interface {
	Build(request *http.Request) concourse.Client
}

type clientFactory struct {
	apiEndpoint string
}

func NewClientFactory(apiEndpoint string) ClientFactory {
	return &clientFactory{
		apiEndpoint: apiEndpoint,
	}
}

func (cf *clientFactory) Build(r *http.Request) concourse.Client {
	transport := authorizationTransport{
		Authorization: r.Header.Get("Authorization"),

		Base: &http.Transport{
			// disable connection pooling
			DisableKeepAlives: true,
		},
	}

	httpClient := &http.Client{
		Transport: transport,
	}

	connection, err := concourse.NewConnection(cf.apiEndpoint, httpClient)
	if err != nil {
		// TODO really just shouldn't have this error case in the first place
		panic(err)
	}

	return concourse.NewClient(connection)
}

type authorizationTransport struct {
	Base          http.RoundTripper
	Authorization string
}

func (transport authorizationTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Authorization", transport.Authorization)
	return transport.Base.RoundTrip(r)
}
