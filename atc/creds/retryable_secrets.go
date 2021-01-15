package creds

import (
	"fmt"
	"time"

	"github.com/concourse/retryhttp"
)

type SecretRetryConfig struct {
	Attempts int           `yaml:"attempts"`
	Interval time.Duration `yaml:"interval"`
}

type RetryableSecrets struct {
	secrets     Secrets
	retryConfig SecretRetryConfig
}

func NewRetryableSecrets(secrets Secrets, retryConfig SecretRetryConfig) Secrets {
	return &RetryableSecrets{secrets: secrets, retryConfig: retryConfig}
}

// Get retrieves the value and expiration of an individual secret
func (rs RetryableSecrets) Get(secretPath string) (interface{}, *time.Time, bool, error) {
	r := &retryhttp.DefaultRetryer{}
	for i := 0; i < rs.retryConfig.Attempts-1; i++ {
		result, expiration, exists, err := rs.secrets.Get(secretPath)
		if err != nil && r.IsRetryable(err) {
			time.Sleep(rs.retryConfig.Interval)
			continue
		}
		return result, expiration, exists, err
	}
	result, expiration, exists, err := rs.secrets.Get(secretPath)
	if err != nil {
		err = fmt.Errorf("%s (after %d retries)", err, rs.retryConfig.Attempts)
	}
	return result, expiration, exists, err
}

// NewSecretLookupPaths defines how variables will be searched in the underlying secret manager
func (rs RetryableSecrets) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []SecretLookupPath {
	return rs.secrets.NewSecretLookupPaths(teamName, pipelineName, allowRootPath)
}
