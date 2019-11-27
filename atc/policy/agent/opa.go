package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/concourse/concourse/atc/policy"
)

type OpaConfig struct {
	URL     string        `long:"opa-url" description:"OPA policy check endpoint."`
	Timeout time.Duration `long:"opa-timeout" default:"5s" description:"OPA request timeout."`
}

func init() {
	policy.RegisterChecker(&OpaConfig{})
}

func (c *OpaConfig) Description() string { return "Open Policy Agent" }
func (c *OpaConfig) IsConfigured() bool  { return c.URL != "" }

func (c *OpaConfig) NewChecker() (policy.Checker, error) {
	return opaChecker{*c}, nil
}

type opaInput struct {
	Input policy.PolicyCheckInput `json:"input"`
}

type opaResult struct {
	Result *bool `json:"result,omitempty"`
}

type opaChecker struct {
	config OpaConfig
}

func (c opaChecker) Check(input policy.PolicyCheckInput) (bool, error) {
	data := opaInput{input}
	jsonBytes, err := json.Marshal(data)

	fmt.Fprintf(os.Stderr, "EVAN: policy_input: %s\n", string(jsonBytes))

	req, err := http.NewRequest("POST", c.config.URL, bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	client.Timeout = c.config.Timeout
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode
	if statusCode != http.StatusOK {
		return false, fmt.Errorf("status: %d", statusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	//result := map[string]interface{}{}
	//err = json.Unmarshal(body, &result)
	//if err != nil {
	//	return false, err
	//}
	//
	//// If no result returned, meaning that the requested policy decision is
	//// undefined OPA, then consider as pass.
	//if result["result"] == nil {
	//	return true, nil
	//}
	//
	//var pass, ok bool
	//if pass, ok = result["result"].(bool); !ok {
	//	return false, fmt.Errorf("bad result returned from opa: %s", result["result"])
	//}

	result := &opaResult{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return false, err
	}

	// If no result returned, meaning that the requested policy decision is
	// undefined OPA, then consider as pass.
	if result.Result == nil {
		return true, nil
	}

	return *result.Result, nil
}
