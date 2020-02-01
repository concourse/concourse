package policy

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
	"sigs.k8s.io/yaml"
	"encoding/json"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/api/accessor"
)

const ActionRunTask = "RunTask"

type Filter struct {
	HttpMethods   []string `long:"policy-check-filter-http-methods" description:"API http method to go through policy check"`
	Actions       []string `long:"policy-check-filter-action" description:"Actions in the list will go through policy check"`
	ActionsToSkip []string `long:"policy-check-filter-action-skip" default:"RunTask" description:"Actions the list will not go through policy check"`
}

func (f Filter) normalize() Filter {
	if len(f.HttpMethods) == 1 {
		f.HttpMethods = strings.Split(f.HttpMethods[0], ",")
	}

	if len(f.Actions) == 1 {
		f.Actions = strings.Split(f.Actions[0], ",")
	}

	if len(f.ActionsToSkip) == 1 {
		f.ActionsToSkip = strings.Split(f.ActionsToSkip[0], ",")
	}

	return f
}

type PolicyCheckInput struct {
	Service        string      `json:"service"`
	ClusterName    string      `json:"cluster_name"`
	ClusterVersion string      `json:"cluster_version"`
	HttpMethod     string      `json:"http_method,omitempty"`
	Action         string      `json:"action"`
	User           string      `json:"user"`
	Team           string      `json:"team,omitempty"`
	Pipeline       string      `json:"pipeline,omitempty"`
	Data           interface{} `json:"data,omitempty"`
}

//go:generate counterfeiter . Checker

// Checker runs filters first, then calls underlying policy agent.
type Checker interface {
	// CheckHttpApi returns true if passes policy check. If not goes through
	// policy check, just return true.
	CheckHttpApi(string, accessor.Access, *http.Request) (bool, error)

	CheckTask(db.Build, atc.TaskConfig) (bool, error)
}

//go:generate counterfeiter . Agent

// Agent should be implemented by policy agents.
type Agent interface {
	// Check returns true if passes policy check. If not goes through policy
	// check, just return true.
	Check(PolicyCheckInput) (bool, error)
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
		return nil, fmt.Errorf("Multiple policy checker configured: %s", strings.Join(checkerDescriptions, ", "))
	}

	for _, factory := range agentFactories {
		if factory.IsConfigured() {
			agent, err := factory.NewAgent(logger.Session("policy-checker"))
			if err != nil {
				return nil, err
			}
			return &checker{
				filter: filter.normalize(),
				agent:  agent,
			}, nil
		}
	}

	// No policy checker configured.
	return nil, nil
}

type checker struct {
	filter Filter
	agent  Agent
}

func (c *checker) CheckHttpApi(action string, acc accessor.Access, req *http.Request) (bool, error) {
	// Ignore self invoked API calls.
	if acc.IsSystem() {
		return true, nil
	}

	// Actions in black will not go through policy check.
	if c.actionToSkip(action) {
		return true, nil
	}

	// Only actions with specified http method will go through policy check.
	// But actions in white list will always go through policy check.
	if !c.httpMethodShouldCheck(req) && !c.actionToCheck(action) {
		return true, nil
	}

	input := PolicyCheckInput{
		Service:        "concourse",
		ClusterName:    clusterName,
		ClusterVersion: clusterVersion,
		HttpMethod:     req.Method,
		Action:         action,
		User:           acc.UserName(),
		Team:           req.FormValue(":team_name"),
		Pipeline:       req.FormValue(":pipeline_name"),
	}

	switch ct := req.Header.Get("Content-type"); ct {
	case "application/json", "text/vnd.yaml", "text/yaml", "text/x-yaml", "application/x-yaml":
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return false, err
		} else if body != nil && len(body) > 0 {
			if ct == "application/json" {
				err = json.Unmarshal(body, &input.Data)
			} else {
				err = yaml.Unmarshal(body, &input.Data)
			}
			if err != nil {
				return false, err
			}

			req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		}
	}

	return c.agent.Check(input)
}

func (c *checker) httpMethodShouldCheck(req *http.Request) bool {
	return inArray(c.filter.HttpMethods, req.Method)
}

func (c *checker) actionToCheck(action string) bool {
	return inArray(c.filter.Actions, action)
}

func (c *checker) actionToSkip(action string) bool {
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

func (c *checker) CheckTask(build db.Build, config atc.TaskConfig) (bool, error) {
	// Actions in skip list will not go through policy check.
	if c.actionToSkip(ActionRunTask) {
		return true, nil
	}

	input := PolicyCheckInput{
		Service:        "concourse",
		ClusterName:    clusterName,
		ClusterVersion: clusterVersion,
		Action:         ActionRunTask,
		Team:           build.TeamName(),
		Pipeline:       build.PipelineName(),
		Data:           config,
	}

	return c.agent.Check(input)
}
