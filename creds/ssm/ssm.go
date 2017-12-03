package ssm

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	varTemplate "github.com/cloudfoundry/bosh-cli/director/template"
)

type Ssm struct {
	api            ssmiface.SSMAPI
	teamName       string
	pipelineName   string
	secretTemplate *template.Template
}

func NewSsm(api ssmiface.SSMAPI, teamName string, pipelineName string, secretTemplate *template.Template) *Ssm {
	return &Ssm{
		api:            api,
		teamName:       teamName,
		pipelineName:   pipelineName,
		secretTemplate: secretTemplate,
	}
}

func (s *Ssm) buildSecretName(varName string) (string, error) {
	var buf bytes.Buffer
	err := s.secretTemplate.Execute(&buf, &SsmSecret{
		Team:     s.teamName,
		Pipeline: s.pipelineName,
		Secret:   varName,
	})
	return buf.String(), err
}

func (s *Ssm) Get(varDef varTemplate.VariableDefinition) (interface{}, bool, error) {
	secretName, err := s.buildSecretName(varDef.Name)
	if err != nil {
		return nil, false, err
	}
	param, err := s.api.GetParameter(&ssm.GetParameterInput{
		Name:           &secretName,
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, false, err
	}
	return *param.Parameter.Value, true, nil
}

func (s *Ssm) List() ([]varTemplate.VariableDefinition, error) {
	secretPath, err := s.buildSecretName("")
	if err != nil {
		return nil, err
	}
	// Remove all trailing slashes
	secretPath = strings.TrimRight(secretPath, "/")
	if secretPath == "" {
		secretPath = "/"
	}
	var varDefs []varTemplate.VariableDefinition
	query := &ssm.GetParametersByPathInput{}
	query = query.SetPath(secretPath).SetRecursive(true).SetWithDecryption(false).SetMaxResults(10)
	err = s.api.GetParametersByPathPages(query, func(page *ssm.GetParametersByPathOutput, lastPage bool) bool {
		for _, param := range page.Parameters {
			varDefs = append(varDefs, varTemplate.VariableDefinition{Name: *param.Name})
		}
		return true
	})
	return varDefs, err
}
