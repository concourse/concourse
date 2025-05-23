package ssm

import (
	"code.cloudfoundry.org/lager/v3"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/concourse/concourse/atc/creds"
)

type ssmFactory struct {
	log             lager.Logger
	api             *ssm.Client
	secretTemplates []*creds.SecretTemplate
	sharedPath      string
}

func NewSsmFactory(log lager.Logger, config aws.Config, secretTemplates []*creds.SecretTemplate, sharedPath string) *ssmFactory {
	return &ssmFactory{
		log:             log,
		api:             ssm.NewFromConfig(config),
		secretTemplates: secretTemplates,
		sharedPath:      sharedPath,
	}
}

func (factory *ssmFactory) NewSecrets() creds.Secrets {
	return NewSsm(factory.log, factory.api, factory.secretTemplates, factory.sharedPath)
}
