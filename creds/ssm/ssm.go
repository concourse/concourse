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

func (s *Ssm) transformSecret(nameTemplate *template.Template, secret string) (string, error) {
	var buf bytes.Buffer
	err := nameTemplate.Execute(&buf, &SsmSecret{
		Team:     s.TeamName,
		Pipeline: s.PipelineName,
		Secret:   secret,
	})
	return buf.String(), err
}

func (s *Ssm) Get(varDef varTemplate.VariableDefinition) (interface{}, bool, error) {
	for _, st := range s.SecretTemplates {
		// Try to get the parameter as string value
		parameter, err := s.transformSecret(st, varDef.Name)
		if err != nil {
			s.log.Error("failed-to-build-ssm-parameter-path-from-secret", err, lager.Data{
				"template": st.Name(),
				"secret":   varDef.Name,
			})
			return nil, false, err
		}
		// If pipeline name is empty, double slashes may be present in the parameter name
		if strings.Contains(parameter, "//") {
			continue
		}
		value, found, err := s.getParameterByName(parameter)
		if err != nil {
			s.log.Error("failed-to-get-ssm-parameter-by-name", err, lager.Data{
				"template":  st.Name(),
				"secret":    varDef.Name,
				"parameter": parameter,
			})
			return nil, false, err
		}
		if found {
			return value, true, nil
		}
		// // Paramter may exist as a complex value so try again using paramter name as root path
		value, found, err = s.getParameterByPath(parameter)
		if err != nil {
			s.log.Error("failed-to-get-ssm-parameter-by-path", err, lager.Data{
				"template":  st.Name(),
				"secret":    varDef.Name,
				"parameter": parameter,
			})
			return nil, false, err
		}
		if found {
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
