package creds

import (
	"fmt"
	"time"

	"github.com/concourse/retryhttp"
)

type SecretRetryConfig struct {
	Attempts int           `long:"secret-retry-attempts" default:"5"  description:"The number of attempts secret will be retried to be fetched, in case a retryable error happens."`
	Interval time.Duration `long:"secret-retry-interval" default:"1s" description:"The interval between secret retry retrieval attempts."`
}

type RetryableSecrets struct {
	secrets     Secrets
	retryConfig SecretRetryConfig
}

func NewRetryableSecrets(secrets Secrets, retryConfig SecretRetryConfig) Secrets {
	return &RetryableSecrets{secrets: secrets, retryConfig: retryConfig}
}

// Get retrieves the value and expiration of an individual secret
func (rs RetryableSecrets) Get(secretPath string) (any, *time.Time, bool, error) {
	return rs.GetWithContext(secretPath, SecretLookupContext{})
}

// NewSecretLookupPaths defines how variables will be searched in the underlying secret manager
func (rs RetryableSecrets) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []SecretLookupPath {
	return rs.NewSecretLookupPathsWithContext(SecretLookupContext{Team: teamName, Pipeline: pipelineName}, allowRootPath)
}

// Get retrieves the value and expiration of an individual secret
func (rs RetryableSecrets) GetWithContext(secretPath string, context SecretLookupContext) (any, *time.Time, bool, error) {
	r := &retryhttp.DefaultRetryer{}
	for i := 0; i < rs.retryConfig.Attempts-1; i++ {
		result, expiration, exists, err := getWithContext(rs.secrets, secretPath, context)
		if err != nil && r.IsRetryable(err) {
			time.Sleep(rs.retryConfig.Interval)
			continue
		}
		return result, expiration, exists, err
	}
	result, expiration, exists, err := getWithContext(rs.secrets, secretPath, context)
	if err != nil {
		err = fmt.Errorf("%s (after %d retries)", err, rs.retryConfig.Attempts)
	}
	return result, expiration, exists, err
}

// NewSecretLookupPaths defines how variables will be searched in the underlying secret manager
func (rs RetryableSecrets) NewSecretLookupPathsWithContext(context SecretLookupContext, allowRootPath bool) []SecretLookupPath {
	return newSecretLookupPathsWithContext(rs.secrets, context, allowRootPath)
}
