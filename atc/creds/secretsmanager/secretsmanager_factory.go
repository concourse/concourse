package secretsmanager

import (
	"text/template"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/concourse/atc/creds"
)

type secretsManagerFactory struct {
	log             lager.Logger
	api             *secretsmanager.SecretsManager
	secretTemplates []*template.Template
}

func NewSecretsManagerFactory(log lager.Logger, session *session.Session, secretTemplates []*template.Template) *secretsManagerFactory {
	return &secretsManagerFactory{
		log:             log,
		api:             secretsmanager.New(session),
		secretTemplates: secretTemplates,
	}
}

func (factory *secretsManagerFactory) NewVariables(teamName string, pipelineName string) creds.Variables {
	return NewSecretsManager(factory.log, factory.api, teamName, pipelineName, factory.secretTemplates)
}
