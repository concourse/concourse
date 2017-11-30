package ssm

import (
	"text/template"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/concourse/atc/creds"
)

type ssmFactory struct {
	session        *session.Session
	secretTemplate *template.Template
}

func NewSsmFactory(session *session.Session, secretTemplate *template.Template) *ssmFactory {
	return &ssmFactory{
		session:        session,
		secretTemplate: secretTemplate,
	}
}

func (factory *ssmFactory) NewVariables(teamName string, pipelineName string) creds.Variables {
	return NewSsm(ssm.New(factory.session), teamName, pipelineName, factory.secretTemplate)
}
