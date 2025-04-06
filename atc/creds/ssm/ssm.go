package ssm

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/concourse/concourse/atc/creds"

	"code.cloudfoundry.org/lager/v3"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . SsmAPI
type SsmAPI interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	GetParametersByPath(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error)
}

type Ssm struct {
	log             lager.Logger
	api             SsmAPI
	secretTemplates []*creds.SecretTemplate
	sharedPath      string
}

func NewSsm(log lager.Logger, api SsmAPI, secretTemplates []*creds.SecretTemplate, sharedPath string) *Ssm {
	return &Ssm{
		log:             log,
		api:             api,
		secretTemplates: secretTemplates,
		sharedPath:      sharedPath,
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
	if s.sharedPath != "" {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(s.sharedPath+"/"))
	}
	return lookupPaths
}

// Get retrieves the value and expiration of an individual secret
func (s *Ssm) Get(secretPath string) (any, *time.Time, bool, error) {
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

func (s *Ssm) getParameterByName(name string) (any, *time.Time, bool, error) {
	ctx := context.TODO()

	param, err := s.api.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: aws.Bool(true),
	})
	if err == nil {
		return *param.Parameter.Value, nil, true, nil
	} else {
		var notFound *types.ParameterNotFound
		if errors.As(err, &notFound) {
			return nil, nil, false, nil
		}
	}
	return nil, nil, false, err
}

func (s *Ssm) getParameterByPath(path string) (any, *time.Time, bool, error) {
	ctx := context.TODO()

	path = strings.TrimRight(path, "/")
	if path == "" {
		path = "/"
	}
	pathQuery := &ssm.GetParametersByPathInput{
		Path:           &path,
		Recursive:      aws.Bool(true),
		WithDecryption: aws.Bool(true),
		MaxResults:     aws.Int32(10),
	}

	value := make(map[string]any)
	paginator := ssm.NewGetParametersByPathPaginator(s.api, pathQuery)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, nil, false, err
		}
		for _, param := range page.Parameters {
			value[(*param.Name)[len(path)+1:]] = *param.Value
		}
	}

	if len(value) == 0 {
		return nil, nil, false, nil
	}
	return value, nil, true, nil
}
