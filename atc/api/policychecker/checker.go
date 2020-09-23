package policychecker

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/db"
	"sigs.k8s.io/yaml"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/policy"
)

//go:generate counterfeiter . PolicyChecker

type PolicyChecker interface {
	Check(string, accessor.Access, *http.Request) (policy.PolicyCheckOutput, error)
}

type checker struct {
	policyChecker *policy.Checker
}

func NewApiPolicyChecker(policyChecker *policy.Checker) PolicyChecker {
	if policyChecker == nil {
		return nil
	}
	return &checker{policyChecker: policyChecker}
}

func (c *checker) Check(action string, acc accessor.Access, req *http.Request) (policy.PolicyCheckOutput, error) {
	// Ignore self invoked API calls.
	if acc.IsSystem() {
		return policy.PassedPolicyCheck(), nil
	}

	// Actions in black will not go through policy check.
	if c.policyChecker.ShouldSkipAction(action) {
		return policy.PassedPolicyCheck(), nil
	}

	// Only actions with specified http method will go through policy check.
	// But actions in white list will always go through policy check.
	if !c.policyChecker.ShouldCheckHttpMethod(req.Method) &&
		!c.policyChecker.ShouldCheckAction(action) {
		return policy.PassedPolicyCheck(), nil
	}

	var (
		teamName     string
		pipelineName string
	)
	if pipeline, ok := req.Context().Value(auth.PipelineContextKey).(db.Pipeline); ok {
		teamName = pipeline.TeamName()
		pipelineName = pipeline.Name()
	} else {
		teamName = req.FormValue(":team_name")
	}
	input := policy.PolicyCheckInput{
		HttpMethod: req.Method,
		Action:     action,
		User:       acc.Claims().UserName,
		Roles:      acc.TeamRoles()[teamName],
		Team:       teamName,
		Pipeline:   pipelineName,
	}

	switch ct := req.Header.Get("Content-type"); ct {
	case "application/json", "text/vnd.yaml", "text/yaml", "text/x-yaml", "application/x-yaml":
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return policy.FailedPolicyCheck(), err
		} else if body != nil && len(body) > 0 {
			if ct == "application/json" {
				err = json.Unmarshal(body, &input.Data)
			} else {
				err = yaml.Unmarshal(body, &input.Data)
			}
			if err != nil {
				return policy.FailedPolicyCheck(), err
			}

			req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		}
	}

	return c.policyChecker.Check(input)
}
