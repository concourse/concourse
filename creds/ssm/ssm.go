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
	api              ssmiface.SSMAPI
	TeamName         string
	PipelineName     string
	SecretTemplate   *template.Template
	FallbackTemplate *template.Template
}

func NewSsm(api ssmiface.SSMAPI, teamName string, pipelineName string, secretTemplate *template.Template, fallbackTemplate *template.Template) *Ssm {
	return &Ssm{
		api:              api,
		TeamName:         teamName,
		PipelineName:     pipelineName,
		SecretTemplate:   secretTemplate,
		FallbackTemplate: fallbackTemplate,
	}
}

func (s *Ssm) buildSecretName(nameTemplate *template.Template, varName string) (string, error) {
	var buf bytes.Buffer
	err := nameTemplate.Execute(&buf, &SsmSecret{
		Team:     s.TeamName,
		Pipeline: s.PipelineName,
		Secret:   varName,
	})
	return buf.String(), err
}

func (s *Ssm) Get(varDef varTemplate.VariableDefinition) (interface{}, bool, error) {
	value, found, err := s.GetVar(s.SecretTemplate, varDef.Name)
	if !found && s.FallbackTemplate != nil {
		value, found, err = s.GetVar(s.FallbackTemplate, varDef.Name)
	}
	return value, found, err
}

func (s *Ssm) GetVar(nameTemplate *template.Template, varName string) (interface{}, bool, error) {
	secretName, err := s.buildSecretName(nameTemplate, varName)
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
	// Merge the secret & fallback variables using a map. Since the default
	// fallback template includes all secret templates as well.
	mergedVars := make(map[string]bool)

	secretVars, err := s.ListVars(s.SecretTemplate)
	if err != nil {
		return nil, err
	}
	for _, v := range secretVars {
		mergedVars[v] = true
	}

	if s.FallbackTemplate != nil {
		fallbackVars, err := s.ListVars(s.FallbackTemplate)
		if err != nil {
			return nil, err
		}

		for _, v := range fallbackVars {
			mergedVars[v] = true
		}
	}

	// Convert to array before returing to caller
	varDefs := make([]varTemplate.VariableDefinition, len(mergedVars))
	i := 0
	for v := range mergedVars {
		varDefs[i] = varTemplate.VariableDefinition{Name: v}
		i++
	}
	return varDefs, nil
}

func (s *Ssm) ListVars(nameTemplate *template.Template) ([]string, error) {
	secretPath, err := s.buildSecretName(nameTemplate, "")
	if err != nil {
		return nil, err
	}
	// Remove all trailing slashes
	secretPath = strings.TrimRight(secretPath, "/")
	if secretPath == "" {
		secretPath = "/"
	}
	var names []string
	query := &ssm.GetParametersByPathInput{}
	query = query.SetPath(secretPath).SetRecursive(true).SetWithDecryption(false).SetMaxResults(10)
	err = s.api.GetParametersByPathPages(query, func(page *ssm.GetParametersByPathOutput, lastPage bool) bool {
		for _, param := range page.Parameters {
			names = append(names, *param.Name)
		}
		return true
	})
	return names, err
}
