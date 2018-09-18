package ssm

import (
	"text/template"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/concourse/atc/creds"
)

type ssmFactory struct {
	log             lager.Logger
	api             *ssm.SSM
	secretTemplates []*template.Template
}

func NewSsmFactory(log lager.Logger, session *session.Session, secretTemplates []*template.Template) *ssmFactory {
	return &ssmFactory{
		log:             log,
		api:             ssm.New(session),
		secretTemplates: secretTemplates,
	}
}

func (factory *ssmFactory) NewVariables(teamName string, pipelineName string) creds.Variables {
	return NewSsm(factory.log, factory.api, teamName, pipelineName, factory.secretTemplates)
}
