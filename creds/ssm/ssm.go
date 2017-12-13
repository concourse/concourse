package ssm

import (
	"bytes"
	"strings"
	"text/template"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	varTemplate "github.com/cloudfoundry/bosh-cli/director/template"
)

type Ssm struct {
	log             lager.Logger
	api             ssmiface.SSMAPI
	TeamName        string
	PipelineName    string
	SecretTemplates []*template.Template
}

func NewSsm(log lager.Logger, api ssmiface.SSMAPI, teamName string, pipelineName string, secretTemplates []*template.Template) *Ssm {
	return &Ssm{
		log:             log,
		api:             api,
		TeamName:        teamName,
		PipelineName:    pipelineName,
		SecretTemplates: secretTemplates,
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
	for _, st := range s.SecretTemplates {
		if secret, err := s.buildSecretName(st, varDef.Name); err != nil {
			s.log.Error("Failed to build SSM parameter path from secret", err, lager.Data{
				"template": st.Name(),
				"secret":   varDef.Name,
			})
			return nil, false, err
		} else if value, found, err := s.getParameterByName(secret); err != nil {
			s.log.Error("Failed to get SSM paramter by name", err, lager.Data{
				"template": st.Name(),
				"secret":   secret,
			})
			return nil, false, err
		} else if found {
			return value, true, nil
		} else if value, found, err = s.getParameterByPath(secret); err != nil {
			s.log.Error("Failed to get SSM paramter by path", err, lager.Data{
				"template": st.Name(),
				"secret":   secret,
			})
			return nil, false, err
		} else if found {
			return value, true, nil
		}
	}
	return nil, false, nil
}

func (s *Ssm) getParameterByName(name string) (interface{}, bool, error) {
	param, err := s.api.GetParameter(&ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: aws.Bool(true),
	})
	if err == nil {
		return *param.Parameter.Value, true, nil

	} else if errObj, ok := err.(awserr.Error); ok && errObj.Code() == ssm.ErrCodeParameterNotFound {
		return nil, false, nil
	}
	return nil, false, err
}

func (s *Ssm) getParameterByPath(path string) (interface{}, bool, error) {
	path = strings.TrimRight(path, "/")
	if path == "" {
		path = "/"
	}
	value := make(map[interface{}]interface{})
	pathQuery := &ssm.GetParametersByPathInput{}
	pathQuery = pathQuery.SetPath(path).SetRecursive(true).SetWithDecryption(true).SetMaxResults(10)
	err := s.api.GetParametersByPathPages(pathQuery, func(page *ssm.GetParametersByPathOutput, lastPage bool) bool {
		for _, param := range page.Parameters {
			value[(*param.Name)[len(path)+1:]] = *param.Value
		}
		return true
	})
	if err != nil {
		return nil, false, err
	}
	if len(value) == 0 {
		return nil, false, nil
	}
	return value, true, nil
}

func (s *Ssm) List() ([]varTemplate.VariableDefinition, error) {
	// not implemented, see vault implementation
	return []varTemplate.VariableDefinition{}, nil
}
