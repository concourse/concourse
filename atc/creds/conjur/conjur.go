package conjur

import (
	"strings"
	"text/template"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
)

type IConjurClient interface {
	RetrieveSecret(string) ([]byte, error)
}

type Conjur struct {
	log             lager.Logger
	client          IConjurClient
	secretTemplates []*template.Template
}

func NewConjur(log lager.Logger, client IConjurClient, secretTemplates []*template.Template) *Conjur {
	return &Conjur{
		log:             log,
		client:          client,
		secretTemplates: secretTemplates,
	}
}

func (c Conjur) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}
	for _, template := range c.secretTemplates {
		c.log.Info(" teamname: " + teamName + "pipeline: " + pipelineName)
		lPath := creds.NewSecretLookupWithTemplate(template, teamName, pipelineName)
		samplePath, err := lPath.VariableToSecretPath("variable")
		if err == nil && !strings.Contains(samplePath, "//") {
			lookupPaths = append(lookupPaths, lPath)
		}
	}

	return lookupPaths
}

func (c Conjur) Get(secretPath string) (interface{}, *time.Time, bool, error) {
	secretValue, err := c.client.RetrieveSecret(secretPath)
	if err != nil {
		return nil, nil, false, nil
	}
	return string(secretValue), nil, true, nil
}
