package policychecker

import (
	"bytes"
	"io"
	"net/http"

	"github.com/bytedance/sonic"
	"sigs.k8s.io/yaml"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/policy"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . PolicyChecker
type PolicyChecker interface {
	Check(string, accessor.Access, *http.Request) (policy.PolicyCheckResult, error)
}

type checker struct {
	policyChecker policy.Checker
}

func NewApiPolicyChecker(policyChecker policy.Checker) PolicyChecker {
	return &checker{policyChecker: policyChecker}
}

func (c *checker) Check(action string, acc accessor.Access, req *http.Request) (policy.PolicyCheckResult, error) {
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

	team := req.URL.Query().Get(":team_name")
	input := policy.PolicyCheckInput{
		HttpMethod: req.Method,
		Action:     action,
		User:       acc.Claims().UserName,
		Roles:      acc.TeamRoles()[team],
		Team:       team,
		Pipeline:   req.FormValue(":pipeline_name"),
	}

	switch ct := req.Header.Get("Content-type"); ct {
	case "application/json", "text/vnd.yaml", "text/yaml", "text/x-yaml", "application/x-yaml":
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		} else if len(body) > 0 {
			if ct == "application/json" {
				err = sonic.Unmarshal(body, &input.Data)
			} else {
				err = yaml.Unmarshal(body, &input.Data)
			}
			if err != nil {
				return nil, err
			}

			req.Body = io.NopCloser(bytes.NewBuffer(body))
		}
	}

	return c.policyChecker.Check(input)
}
