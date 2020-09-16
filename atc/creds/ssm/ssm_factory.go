package ssm

import (
	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/concourse/concourse/atc/creds"
)

type ssmFactory struct {
	log             lager.Logger
	api             *ssm.SSM
	secretTemplates []*creds.SecretTemplate
}

func NewSsmFactory(log lager.Logger, session *session.Session, secretTemplates []*creds.SecretTemplate) *ssmFactory {
	return &ssmFactory{
		log:             log,
		api:             ssm.New(session),
		secretTemplates: secretTemplates,
	}
}

func (factory *ssmFactory) NewSecrets() creds.Secrets {
	return NewSsm(factory.log, factory.api, factory.secretTemplates)
}
