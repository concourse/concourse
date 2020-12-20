package http

import (
	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/creds"
)

type httpFactory struct {
	log lager.Logger
	url string
}

func NewHTTPFactory(log lager.Logger, url string) *httpFactory {
	return &httpFactory{
		log: log,
		url: url,
	}
}

func (factory *httpFactory) NewSecrets() creds.Secrets {
	return &HTTPSecretManager{
		URL: factory.url,
	}
}
