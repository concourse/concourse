package policy

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

const ActionUseImage = "UseImage"
const ActionRunSetPipeline = "SetPipeline"

type PolicyCheckNotPass struct {
	Messages []string
}

func (e PolicyCheckNotPass) Error() string {
	if len(e.Messages) == 0 {
		return "policy check failed"
	}
	lines := []string{""}
	lines = append(lines, e.Messages...)
	return fmt.Sprintf("policy check failed: %s", strings.Join(lines, "\n * "))
}

type Filter struct {
	HttpMethods   []string `long:"policy-check-filter-http-method" description:"API http method to go through policy check"`
	Actions       []string `long:"policy-check-filter-action" description:"Actions in the list will go through policy check"`
	ActionsToSkip []string `long:"policy-check-filter-action-skip" description:"Actions the list will not go through policy check"`
}

type PolicyCheckInput struct {
	Service        string      `json:"service"`
	ClusterName    string      `json:"cluster_name"`
	ClusterVersion string      `json:"cluster_version"`
	HttpMethod     string      `json:"http_method,omitempty"`
	Action         string      `json:"action"`
	User           string      `json:"user,omitempty"`
	Team           string      `json:"team,omitempty"`
	Roles          []string    `json:"roles,omitempty"`
	Pipeline       string      `json:"pipeline,omitempty"`
	Data           interface{} `json:"data,omitempty"`
}

//counterfeiter:generate . PolicyCheckResult
type PolicyCheckResult interface {
	Allowed() bool
	ShouldBlock() bool
	Messages() []string
}

type internalPolicyCheckResult struct {
	allowed  bool
	messages []string
}

func (r internalPolicyCheckResult) Allowed() bool {
	return r.allowed
}

func (r internalPolicyCheckResult) ShouldBlock() bool {
	return !r.allowed
}

func (r internalPolicyCheckResult) Messages() []string {
	return r.messages
}

// PassedPolicyCheck creates a generic passed check
func PassedPolicyCheck() PolicyCheckResult {
	return internalPolicyCheckResult{
		allowed:  true,
		messages: []string{""},
	}
}

//counterfeiter:generate . Agent

// Agent should be implemented by policy agents.
type Agent interface {
	// Check returns true if passes policy check. If not goes through policy
	// check, just return true.
	Check(PolicyCheckInput) (PolicyCheckResult, error)
}

//counterfeiter:generate . AgentFactory
type AgentFactory interface {
	Description() string
	IsConfigured() bool
	NewAgent(lager.Logger) (Agent, error)
}

var agentFactories []AgentFactory

func RegisterAgent(factory AgentFactory) {
	agentFactories = append(agentFactories, factory)
}

func WireCheckers(group *flags.Group) {
	for _, factory := range agentFactories {
		_, err := group.AddGroup(fmt.Sprintf("Policy Check Agent (%s)", factory.Description()), "", factory)
		if err != nil {
			panic(err)
		}
	}
}

var (
	clusterName    string
	clusterVersion string
)

//counterfeiter:generate . Checker
type Checker interface {
	ShouldCheckHttpMethod(string) bool
	ShouldCheckAction(string) bool
	ShouldSkipAction(string) bool

	Check(input PolicyCheckInput) (PolicyCheckResult, error)
}

func Initialize(logger lager.Logger, cluster string, version string, filter Filter) (Checker, error) {
	logger.Debug("policy-checker-initialize")

	clusterName = cluster
	clusterVersion = version

	var checkerDescriptions []string
	for _, factory := range agentFactories {
		if factory.IsConfigured() {
			checkerDescriptions = append(checkerDescriptions, factory.Description())
		}
	}
	if len(checkerDescriptions) > 1 {
		return nil, fmt.Errorf("multiple policy checker configured: %s", strings.Join(checkerDescriptions, ", "))
	}

	for _, factory := range agentFactories {
		if factory.IsConfigured() {
			agent, err := factory.NewAgent(logger.Session("policy-checker"))
			if err != nil {
				return nil, err
			}

			logger.Info("warning-experiment-policy-check",
				lager.Data{"rfc": "https://github.com/concourse/rfcs/pull/41"})

			return &AgentChecker{
				filter: filter,
				agent:  agent,
			}, nil
		}
	}

	// No policy checker configured.
	return NoopChecker{}, nil
}

type AgentChecker struct {
	filter Filter
	agent  Agent
}

func (c *AgentChecker) ShouldCheckHttpMethod(method string) bool {
	return inArray(c.filter.HttpMethods, method)
}

func (c *AgentChecker) ShouldCheckAction(action string) bool {
	return inArray(c.filter.Actions, action)
}

func (c *AgentChecker) ShouldSkipAction(action string) bool {
	return inArray(c.filter.ActionsToSkip, action)
}

func inArray(array []string, target string) bool {
	found := false
	for _, ele := range array {
		if ele == target {
			found = true
			break
		}
	}
	return found
}

func (c *AgentChecker) Check(input PolicyCheckInput) (PolicyCheckResult, error) {
	input.Service = "concourse"
	input.ClusterName = clusterName
	input.ClusterVersion = clusterVersion
	return c.agent.Check(input)
}

type NoopChecker struct{}

func (noop NoopChecker) ShouldCheckHttpMethod(string) bool { return false }
func (noop NoopChecker) ShouldCheckAction(string) bool     { return false }
func (noop NoopChecker) ShouldSkipAction(string) bool      { return true }

func (noop NoopChecker) Check(PolicyCheckInput) (PolicyCheckResult, error) {
	return PassedPolicyCheck(), nil
}
