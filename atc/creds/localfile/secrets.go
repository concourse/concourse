package localfile

import (
	"code.cloudfoundry.org/lager"
	"encoding/json"
	"fmt"
	"github.com/concourse/concourse/atc/creds"
	"github.com/thedevsaddam/gojsonq"
	yaml "sigs.k8s.io/yaml"
	"time"
)

type Secrets struct {
	path   string
	logger lager.Logger
}

func (secrets *Secrets) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}

	// Team + Pipeline Credentails
	if len(pipelineName) > 0 {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix("teams."+teamName+"."+pipelineName+"."))
	}

	// Team credentails
	lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix("teams."+teamName+"."))

	// Global Credentials
	lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix("shared."))

	return lookupPaths
}

func (secrets *Secrets) Get(secretPath string) (interface{}, *time.Time, bool, error) {

	secrets.logger.Info("secrets-get-path", lager.Data{
		"path": secrets.path,
	})

	jq := gojsonq.New(gojsonq.SetDecoder(&yamlDecoder{})).File(secrets.path)

	query := jq.Find("shared.some_key")

	secrets.logger.Info("secrets-get", lager.Data{
		"test-query": fmt.Sprintf("%v", query),
	})

	query2 := jq.Find("shared.some_key")

	secrets.logger.Info(secretPath, lager.Data{
		"test-query": fmt.Sprintf("%v", query2),
	})

	/*
		result, err := jq.FindR(secretPath)

		if err != nil {

			secrets.logger.Info("secretes-get-error", lager.Data{
				"error": fmt.Sprintf("%v", err),
			})

			// TODO: Figure out how to handle real errors and not just 404
			return nil, nil, false, nil
		}

		secrets.logger.Info("secretes-get-error", lager.Data{
			"error": "did not return",
		})

		value, _ := result.String()
	*/

	result := jq.Find(secretPath)

	if result == nil {
		return nil, nil, false, nil
	}

	return fmt.Sprintf("%v", result), nil, true, nil
}

type yamlDecoder struct{}

func (i *yamlDecoder) Decode(data []byte, v interface{}) error {
	bb, err := yaml.YAMLToJSON(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(bb, &v)
}
