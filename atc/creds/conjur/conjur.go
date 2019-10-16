package conjur

import (
	"bytes"
	"text/template"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
	"github.com/cyberark/conjur-api-go/conjurapi"
)

type Conjur struct {
	log             lager.Logger
	client          *conjurapi.Client
	secretTemplates []*template.Template
}

func NewConjur(log lager.Logger, client *conjurapi.Client, secretTemplates []*template.Template) *Conjur {
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

func (n Conjur) NewSecretLookupPaths(teamName string, pipelineName string) []creds.SecretLookupPath {

	lookupPaths := []creds.SecretLookupPath{}
	for _, template := range n.secretTemplates {
		lPath := &SecretLookupPathConjur{
			NameTemplate: template,
			TeamName:     teamName,
			PipelineName: pipelineName,
		}
		lookupPaths = append(lookupPaths, lPath)
	}
	return lookupPaths
}

func (c Conjur) Get(secretPath string) (interface{}, *time.Time, bool, error) {
	secretValue, err := c.client.RetrieveSecret(secretPath)
	if err != nil {
		c.log.Error("error-retrieving-secret", err)
		return nil, nil, false, err
	}
	return string(secretValue), nil, true, err
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
