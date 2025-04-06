package secretsmanager

import (
	lager "code.cloudfoundry.org/lager/v3"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/concourse/concourse/atc/creds"
)

type secretsManagerFactory struct {
	log             lager.Logger
	api             *secretsmanager.Client
	secretTemplates []*creds.SecretTemplate
}

func NewSecretsManagerFactory(log lager.Logger, config aws.Config, secretTemplates []*creds.SecretTemplate) *secretsManagerFactory {
	return &secretsManagerFactory{
		log:             log,
		api:             secretsmanager.NewFromConfig(config),
		secretTemplates: secretTemplates,
	}
}

func (factory *secretsManagerFactory) NewSecrets() creds.Secrets {
	return NewSecretsManager(factory.log, factory.api, factory.secretTemplates)
}
