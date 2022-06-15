package ssm

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/concourse/concourse/atc/creds"
	"gopkg.in/yaml.v2"

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
func (s *Ssm) Get(secretPath string) (interface{}, *time.Time, bool, error) {
	// Try to get the parameter as string value, by name
	value, expiration, found, err := s.getParameterByName(secretPath)

	if err != nil {
		s.log.Error("unable to retrieve aws ssm secret by name", err, lager.Data{
			"secretPath": secretPath,
		})
		return nil, nil, false, err
	}
	if found {
		return value, expiration, true, nil
	}
	// Parameter may exist as a complex value so try again using parameter name as root path
	value, expiration, found, err = s.getParameterByPath(secretPath)
	if err != nil {
		s.log.Error("unable to retrieve aws ssm secret by path", err, lager.Data{
			"secretPath": secretPath,
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
		format, err := s.getParameterFormat(&name)
		if err != nil {
			return nil, nil, false, err
		}

		value, err := unmarshalParameterFormat(*param.Parameter.Value, format)
		if err != nil {
			return nil, nil, false, err
		}

		return value, nil, true, nil
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

func isValidFormat(format string) bool {
	switch format {
	case "raw", "trim", "yml", "yaml", "json":
		return true
	}
	return false
}

func (s *Ssm) getParameterFormat(id *string) (string, error) {
	format := "raw"

	tags, err := s.api.ListTagsForResource(&ssm.ListTagsForResourceInput{
		ResourceType: aws.String(ssm.ResourceTypeForTaggingParameter),
		ResourceId:   id,
	})

	if err == nil {
		for i := range tags.TagList {
			tag := strings.ToLower(*tags.TagList[i].Key)
			if tag == "format" {
				format = strings.ToLower(*tags.TagList[i].Value)
			}
		}

		if !isValidFormat(format) {
			return "nil", fmt.Errorf("invalid format %s", format)
		}
	}

	return format, nil
}

func unmarshalParameterFormat(input string, format string) (interface{}, error) {
	var value interface{}

	switch format {
	case "json":
		err := json.Unmarshal([]byte(input), &value)
		if err != nil {
			return nil, err
		}
	case "yml", "yaml":
		err := yaml.Unmarshal([]byte(input), &value)
		if err != nil {
			return nil, err
		}
	case "trim":
		value = strings.TrimSpace(string(input))
	case "raw":
		value = string(input)
	default:
		return nil, fmt.Errorf("unknown format %s, should never happen, ", format)
	}

	return value, nil
}
