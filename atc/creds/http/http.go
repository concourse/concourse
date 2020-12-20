package http

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/concourse/concourse/atc/creds"

	"sigs.k8s.io/yaml"
)

type HTTPSecretManager struct {
	URL string
}

// NewSecretLookupPaths defines how variables will be searched in the underlying secret manager
func (h HTTPSecretManager) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}

	if len(pipelineName) > 0 {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(path.Join(teamName, pipelineName)+"/"))
	}

	lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(path.Join(teamName)+"/"))

	if allowRootPath {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix("/"))
	}

	return lookupPaths
}

// Get retrieves the value and expiration of an individual secret
func (h HTTPSecretManager) Get(secretPath string) (interface{}, *time.Time, bool, error) {
	url := h.URL
	if !strings.HasPrefix(secretPath, "/") {
		url += "/"
	}
	url += secretPath

	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return nil, nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil, false, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, false, fmt.Errorf(
			"Expected either 200 or 404, received %d", resp.StatusCode,
		)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, false, err
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))

	if contentType == "application/x-yaml" || contentType == "text/yaml" {
		var secret interface{}
		if err := yaml.Unmarshal(body, &secret); err != nil {
			return nil, nil, false, err
		}
		return secret, nil, true, nil
	} else if contentType == "application/json" {
		var secret interface{}
		if err := json.Unmarshal(body, &secret); err != nil {
			return nil, nil, false, err
		}
		return secret, nil, true, nil
	}

	return string(body), nil, true, nil
}
