package secretsmanager

import (
	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/concourse/concourse/atc/creds"
)

type secretsManagerFactory struct {
	log             lager.Logger
	api             *secretsmanager.SecretsManager
	secretTemplates []*creds.SecretTemplate
}

func NewSecretsManagerFactory(log lager.Logger, session *session.Session, secretTemplates []*creds.SecretTemplate) *secretsManagerFactory {
	return &secretsManagerFactory{
		log:             log,
		api:             secretsmanager.New(session),
		secretTemplates: secretTemplates,
	}
}

func (factory *secretsManagerFactory) NewSecrets() creds.Secrets {
	return NewSecretsManager(factory.log, factory.api, factory.secretTemplates)
}
