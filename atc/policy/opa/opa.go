package opa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager/v3"

	"github.com/concourse/concourse/atc/policy"
)

type OpaConfig struct {
	URL                  string        `long:"opa-url" description:"OPA policy check endpoint."`
	Timeout              time.Duration `long:"opa-timeout" default:"5s" description:"OPA request timeout."`
	ResultAllowedKey     string        `long:"opa-result-allowed-key" description:"Key name of if pass policy check in OPA returned result. Expects a boolean value." default:"result.allowed"`
	ResultShouldBlockKey string        `long:"opa-result-should-block-key" description:"Key name of if should block current action in OPA returned result. Expects a boolean value."  default:"result.block"`
	ResultMessagesKey    string        `long:"opa-result-messages-key" description:"Key name of messages in OPA returned result." default:"result.reasons"`
}

func init() {
	policy.RegisterAgent(&OpaConfig{})
}

func (c *OpaConfig) Description() string { return "Open Policy Agent" }
func (c *OpaConfig) IsConfigured() bool  { return c.URL != "" }

func (c *OpaConfig) NewAgent(logger lager.Logger) (policy.Agent, error) {
	return opa{*c, logger}, nil
}

type opaInput struct {
	Input policy.PolicyCheckInput `json:"input"`
}

type opa struct {
	config OpaConfig
	logger lager.Logger
}

func (c opa) Check(input policy.PolicyCheckInput) (policy.PolicyCheckResult, error) {
	data := opaInput{input}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	c.logger.Debug("opa-check", lager.Data{"input": string(jsonBytes)})

	req, err := http.NewRequest("POST", c.config.URL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	client.Timeout = c.config.Timeout
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OPA server: connecting: %w", err)
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("OPA server returned status: %d", statusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("OPA server: reading response body: %s", err.Error())
	}

	result, err := ParseOpaResult(body, c.config)
	if err != nil {
		return nil, fmt.Errorf("parsing OPA results: %w", err)
	}

	return result, nil
}
