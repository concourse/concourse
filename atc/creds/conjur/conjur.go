package conjur

import (
	"time"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/creds"
)

type IConjurClient interface {
	RetrieveSecret(string) ([]byte, error)
}

type Conjur struct {
	log             lager.Logger
	client          IConjurClient
	secretTemplates []*creds.SecretTemplate
	sharedPath      string
}

func NewConjur(log lager.Logger, client IConjurClient, secretTemplates []*creds.SecretTemplate, sharedPath string) *Conjur {
	return &Conjur{
		log:             log,
		client:          client,
		secretTemplates: secretTemplates,
		sharedPath:      sharedPath,
	}
}

func (c Conjur) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}
	for _, template := range c.secretTemplates {
		c.log.Info(" teamname: " + teamName + "pipeline: " + pipelineName)
		if lPath := creds.NewSecretLookupWithTemplate(template, teamName, pipelineName); lPath != nil {
			lookupPaths = append(lookupPaths, lPath)
		}
	}
	if c.sharedPath != "" {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(c.sharedPath+"/"))
	}

	return lookupPaths
}

func (c Conjur) Get(secretPath string) (any, *time.Time, bool, error) {
	secretValue, err := c.client.RetrieveSecret(secretPath)
	if err != nil {
		return nil, nil, false, nil
	}
	return string(secretValue), nil, true, nil
}
