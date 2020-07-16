package ssm

import (
	"strings"
	"time"

	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/vars"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

type Ssm struct {
	log             lager.Logger
	api             ssmiface.SSMAPI
	secretTemplates []*creds.SecretTemplate
}

func NewSsm(log lager.Logger, api ssmiface.SSMAPI, secretTemplates []*creds.SecretTemplate) *Ssm {
	return &Ssm{
		log:             log,
		api:             api,
		secretTemplates: secretTemplates,
	}
}

// NewSecretLookupPaths defines how variables will be searched in the underlying secret manager
func (s *Ssm) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}
	for _, tmpl := range s.secretTemplates {
		if lPath := creds.NewSecretLookupWithTemplate(tmpl, teamName, pipelineName); lPath != nil {
			lookupPaths = append(lookupPaths, lPath)
		}
	}
	return lookupPaths
}

// Get retrieves the value and expiration of an individual secret
func (s *Ssm) Get(ref vars.VariableReference) (interface{}, *time.Time, bool, error) {
	// Try to get the parameter as string value, by name
	value, expiration, found, err := s.getParameterByName(ref.Name)
	if err != nil {
		s.log.Error("unable to retrieve aws ssm secret by name", err, lager.Data{
			"secretPath": ref.Name,
		})
		return nil, nil, false, err
	}
	if found {
		return value, expiration, true, nil
	}
	// Parameter may exist as a complex value so try again using parameter name as root path
	value, expiration, found, err = s.getParameterByPath(ref.Name)
	if err != nil {
		s.log.Error("unable to retrieve aws ssm secret by path", err, lager.Data{
			"secretPath": ref.Name,
		})
		return nil, nil, false, err
	}
	if found {
		return value, expiration, true, nil
	}
	return nil, nil, false, nil
}

func (s *Ssm) getParameterByName(name string) (interface{}, *time.Time, bool, error) {
	param, err := s.api.GetParameter(&ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: aws.Bool(true),
	})
	if err == nil {
		return *param.Parameter.Value, nil, true, nil

	} else if errObj, ok := err.(awserr.Error); ok && errObj.Code() == ssm.ErrCodeParameterNotFound {
		return nil, nil, false, nil
	}
	return nil, nil, false, err
}

func (s *Ssm) getParameterByPath(path string) (interface{}, *time.Time, bool, error) {
	path = strings.TrimRight(path, "/")
	if path == "" {
		path = "/"
	}
	value := make(map[string]interface{})
	pathQuery := &ssm.GetParametersByPathInput{}
	pathQuery = pathQuery.SetPath(path).SetRecursive(true).SetWithDecryption(true).SetMaxResults(10)
	err := s.api.GetParametersByPathPages(pathQuery, func(page *ssm.GetParametersByPathOutput, lastPage bool) bool {
		for _, param := range page.Parameters {
			value[(*param.Name)[len(path)+1:]] = *param.Value
		}
		return true
	})
	if err != nil {
		return nil, nil, false, err
	}
	if len(value) == 0 {
		return nil, nil, false, nil
	}
	return value, nil, true, nil
}

