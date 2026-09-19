package conjur

import (
	"time"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/creds"
)

type IConjurClient interface {
	RetrieveSecret(string) ([]byte, error)
}

var _ creds.Secrets = (*Conjur)(nil)

type Conjur struct {
	log             lager.Logger
	client          IConjurClient
	secretTemplates []*creds.SecretTemplate
}

func NewConjur(log lager.Logger, client IConjurClient, secretTemplates []*creds.SecretTemplate) *Conjur {
	return &Conjur{
		log:             log,
		client:          client,
		secretTemplates: secretTemplates,
	}
}

func (c Conjur) NewSecretLookupPaths(params creds.SecretLookupParams, allowRootPath bool) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}
	for _, template := range c.secretTemplates {
		if lPath := creds.NewSecretLookupWithTemplate(template, params.Team, params.Pipeline); lPath != nil {
			lookupPaths = append(lookupPaths, lPath)
		}
	}

	return lookupPaths
}

func (c Conjur) Get(secretPath string, _ creds.SecretLookupParams) (any, *time.Time, bool, error) {
	secretValue, err := c.client.RetrieveSecret(secretPath)
	if err != nil {
		return nil, nil, false, nil
	}
	return string(secretValue), nil, true, nil
}
