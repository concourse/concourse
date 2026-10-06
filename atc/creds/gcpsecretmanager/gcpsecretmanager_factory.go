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
	requestTimeout  time.Duration
	secretTemplates []*creds.SecretTemplate
	delimiter       string
}

func NewSecretManagerFactory(
	log lager.Logger,
	api SecretManagerAPI,
	projectID string,
	requestTimeout time.Duration,
	secretTemplates []*creds.SecretTemplate,
	delimiter string,
) *secretManagerFactory {
	return &secretManagerFactory{
		log:             log,
		api:             api,
		projectID:       projectID,
		requestTimeout:  requestTimeout,
		secretTemplates: secretTemplates,
		delimiter:       delimiter,
	}
}

func (factory *secretManagerFactory) NewSecrets() creds.Secrets {
	return NewSecretManager(
		factory.log,
		factory.api,
		factory.projectID,
		factory.requestTimeout,
		factory.secretTemplates,
		factory.delimiter,
	)
}
