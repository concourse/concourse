package http

import (
	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/creds"
)

type httpFactory struct {
	log lager.Logger
	url string

	basicAuthUsername string
	basicAuthPassword string
}

func NewHTTPFactory(
	log lager.Logger, url string,
	basicAuthUsername, basicAuthPassword string,
) *httpFactory {
	return &httpFactory{
		log: log,
		url: url,

		basicAuthUsername: basicAuthUsername,
		basicAuthPassword: basicAuthPassword,
	}
}

func (factory *httpFactory) NewSecrets() creds.Secrets {
	return &HTTPSecretManager{
		URL: factory.url,

		BasicAuthUsername: factory.basicAuthUsername,
		BasicAuthPassword: factory.basicAuthPassword,
	}
}
