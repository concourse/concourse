package conjur

import (
	"text/template"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/creds"
	"github.com/cyberark/conjur-api-go/conjurapi"
)

type conjurFactory struct {
	log             lager.Logger
	client          *conjurapi.Client
	secretTemplates []*template.Template
}

func NewConjurFactory(log lager.Logger, client *conjurapi.Client, secretTemplates []*template.Template) *conjurFactory {
	return &conjurFactory{
		log:             log,
		client:          client,
		secretTemplates: secretTemplates,
	}
}

func (factory *conjurFactory) NewSecrets() creds.Secrets {
	return NewConjur(factory.log, factory.client, factory.secretTemplates)
}
