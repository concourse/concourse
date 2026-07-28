package conjur

import (
	"code.cloudfoundry.org/lager/v3"

	"github.com/concourse/concourse/atc/creds"
	"github.com/cyberark/conjur-api-go/conjurapi"
)

type conjurFactory struct {
	log             lager.Logger
	client          *conjurapi.Client
	secretTemplates []*creds.SecretTemplate
	sharedPath      string
}

func NewConjurFactory(log lager.Logger, client *conjurapi.Client, secretTemplates []*creds.SecretTemplate, sharedPath string) *conjurFactory {
	return &conjurFactory{
		log:             log,
		client:          client,
		secretTemplates: secretTemplates,
		sharedPath:      sharedPath,
	}
}

func (factory *conjurFactory) NewSecrets() creds.Secrets {
	return NewConjur(factory.log, factory.client, factory.secretTemplates, factory.sharedPath)
}
