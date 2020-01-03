package localfile

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
	"github.com/tidwall/gjson"
	"io/ioutil"
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
		"path":   secrets.path,
		"secret": secretPath,
	})

	// TODO: Not efficient to read it every time, though it does allow updating secrets without a restart.
	// Could check mtime to reload file or something...
	yamlDoc, _ := ioutil.ReadFile(secrets.path)
	jsonDoc, _ := yaml.YAMLToJSON(yamlDoc)

	result := gjson.GetBytes(jsonDoc, secretPath)

	if !result.Exists() {
		return nil, nil, false, nil
	}

	// TODO: Would basically like string or map (array seems unecessary/excessive to handle).
	// Do not want a nested map (excessive/unecessary).
	return result.Value(), nil, true, nil
}
