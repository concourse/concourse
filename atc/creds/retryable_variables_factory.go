package creds

import (
	"fmt"
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/retryhttp"
	"time"
)

type SecretRetryConfig struct {
	Attempts int           `long:"secret-retry-attempts" default:"5"  description:"The number of attempts secret will be retried to be fetched, in case a retryable error happens."`
	Interval time.Duration `long:"secret-retry-interval" default:"1s" description:"The interval between secret retry retrieval attempts."`
}

type RetryableVariablesFactory struct {
	factory     VariablesFactory
	retryConfig SecretRetryConfig
}

type RetryableVariables struct {
	variables   Variables
	retryConfig SecretRetryConfig
}

func NewRetryableVariablesFactory(factory VariablesFactory, retryConfig SecretRetryConfig) VariablesFactory {
	return &RetryableVariablesFactory{factory: factory, retryConfig: retryConfig}
}

func (rvf RetryableVariablesFactory) NewVariables(teamName string, pipelineName string) Variables {
	return RetryableVariables{variables: rvf.factory.NewVariables(teamName, pipelineName), retryConfig: rvf.retryConfig}
}

func (rv RetryableVariables) Get(varDef template.VariableDefinition) (interface{}, bool, error) {
	r := &retryhttp.DefaultRetryer{}
	for i := 0; i < rv.retryConfig.Attempts-1; i++ {
		result, exists, err := rv.variables.Get(varDef)
		if err != nil && r.IsRetryable(err) {
			time.Sleep(rv.retryConfig.Interval)
			continue
		}
		return result, exists, err
	}
	result, exists, err := rv.variables.Get(varDef)
	if err != nil {
		err = fmt.Errorf("%s (after %d retries)", err, rv.retryConfig.Attempts)
	}
	return result, exists, err
}

func (rv RetryableVariables) List() ([]template.VariableDefinition, error) {
	r := &retryhttp.DefaultRetryer{}
	for i := 0; i < rv.retryConfig.Attempts-1; i++ {
		result, err := rv.variables.List()
		if err != nil && r.IsRetryable(err) {
			time.Sleep(rv.retryConfig.Interval)
			continue
		}
		return result, err
	}
	result, err := rv.variables.List()
	if err != nil {
		err = fmt.Errorf("%s (after %d retries)", err, rv.retryConfig.Attempts)
	}
	return result, err
}
