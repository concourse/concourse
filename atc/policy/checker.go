package policy

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
)

const ActionUseImage = "UseImage"

type PolicyCheckNotPass struct {
	Reasons []string
}

func (e PolicyCheckNotPass) Error() string {
	return fmt.Sprintf("policy check failed: %s", strings.Join(e.Reasons, ", "))
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

type PolicyCheckOutput struct {
	Allowed bool
	Reasons []string
}

// FailedPolicyCheck creates a generic failed check
func FailedPolicyCheck() PolicyCheckOutput {
	return PolicyCheckOutput{
		Allowed: false,
		Reasons: []string{},
	}
}

// PassedPolicyCheck creates a generic passed check
func PassedPolicyCheck() PolicyCheckOutput {
	return PolicyCheckOutput{
		Allowed: true,
		Reasons: []string{},
	}
}

//go:generate counterfeiter . Agent

// Agent should be implemented by policy agents.
type Agent interface {
	// Check returns true if passes policy check. If not goes through policy
	// check, just return true.
	Check(PolicyCheckInput) (PolicyCheckOutput, error)
}

//go:generate counterfeiter . AgentFactory

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

func Initialize(logger lager.Logger, cluster string, version string, filter Filter) (*Checker, error) {
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
		return nil, fmt.Errorf("Multiple policy checker configured: %s", strings.Join(checkerDescriptions, ", "))
	}

	for _, factory := range agentFactories {
		if factory.IsConfigured() {
			agent, err := factory.NewAgent(logger.Session("policy-checker"))
			if err != nil {
				return nil, err
			}

			logger.Info("warning-experiment-policy-check",
				lager.Data{"rfc": "https://github.com/concourse/rfcs/pull/41"})

			return &Checker{
				filter: filter,
				agent:  agent,
			}, nil
		}
	}

	// No policy checker configured.
	return nil, nil
}

type Checker struct {
	filter Filter
	agent  Agent
}

func (c *Checker) ShouldCheckHttpMethod(method string) bool {
	return inArray(c.filter.HttpMethods, method)
}

func (c *Checker) ShouldCheckAction(action string) bool {
	return inArray(c.filter.Actions, action)
}

func (c *Checker) ShouldSkipAction(action string) bool {
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

func (c *Checker) Check(input PolicyCheckInput) (PolicyCheckOutput, error) {
	input.Service = "concourse"
	input.ClusterName = clusterName
	input.ClusterVersion = clusterVersion
	return c.agent.Check(input)
}
