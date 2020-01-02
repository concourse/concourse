package conjur

import (
	"bytes"
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

// SecretLookupPathConjur is an implementation which returns an evaluated go text template
type SecretLookupPathConjur struct {
	NameTemplate *template.Template
	TeamName     string
	PipelineName string
}

func (c Conjur) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}
	for _, template := range c.secretTemplates {
		c.log.Info(" teamname: " + teamName + "pipeline: " + pipelineName)

		lPath := &SecretLookupPathConjur{
			NameTemplate: template,
			TeamName:     teamName,
			PipelineName: pipelineName,
		}

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

func (sl SecretLookupPathConjur) VariableToSecretPath(varName string) (string, error) {
	var buf bytes.Buffer
	err := sl.NameTemplate.Execute(&buf, &Secret{
		Team:     sl.TeamName,
		Pipeline: sl.PipelineName,
		Secret:   varName,
	})
	return buf.String(), err
}
