package gcpsecretmanager

import (
	"time"

	lager "code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/v8/atc/creds"
)

type secretManagerFactory struct {
	log             lager.Logger
	api             SecretManagerAPI
	projectID       string
	requestTimeout  time.Duration
	secretTemplates []*creds.SecretTemplate
	unwrapValue     bool
}

func NewSecretManagerFactory(
	log lager.Logger,
	api SecretManagerAPI,
	projectID string,
	requestTimeout time.Duration,
	secretTemplates []*creds.SecretTemplate,
	unwrapValue bool,
) *secretManagerFactory {
	return &secretManagerFactory{
		log:             log,
		api:             api,
		projectID:       projectID,
		requestTimeout:  requestTimeout,
		secretTemplates: secretTemplates,
		unwrapValue:     unwrapValue,
	}
}

func (factory *secretManagerFactory) NewSecrets() creds.Secrets {
	return NewSecretManager(
		factory.log,
		factory.api,
		factory.projectID,
		factory.requestTimeout,
		factory.secretTemplates,
		factory.unwrapValue,
	)
}
