package gcpsecretmanager

import (
	"time"

	lager "code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/creds"
)

type secretManagerFactory struct {
	log             lager.Logger
	api             SecretManagerAPI
	projectID       string
	secretVersion   string
	requestTimeout  time.Duration
	secretTemplates []*creds.SecretTemplate
}

func NewSecretManagerFactory(
	log lager.Logger,
	api SecretManagerAPI,
	projectID string,
	secretVersion string,
	requestTimeout time.Duration,
	secretTemplates []*creds.SecretTemplate,
) *secretManagerFactory {
	return &secretManagerFactory{
		log:             log,
		api:             api,
		projectID:       projectID,
		secretVersion:   secretVersion,
		requestTimeout:  requestTimeout,
		secretTemplates: secretTemplates,
	}
}

func (factory *secretManagerFactory) NewSecrets() creds.Secrets {
	return NewSecretManager(
		factory.log,
		factory.api,
		factory.projectID,
		factory.secretVersion,
		factory.requestTimeout,
		factory.secretTemplates,
	)
}
