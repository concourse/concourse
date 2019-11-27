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

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

const ActionRunTask = "RunTask"

type Filter struct {
	HttpMethods     []string `long:"policy-check-filter-http-methods" description:"API http method to go through policy check"`
	ActionWhiteList []string `long:"policy-check-filter-action-white-list" description:"Actions in white list will go through policy check"`
	ActionBlackList []string `long:"policy-check-filter-action-black-list" default:"RunTask" description:"Actions in black list will not go through policy check"`
}

func (f Filter) normalize() Filter {
	if len(f.ActionWhiteList) == 1 {
		f.ActionWhiteList = strings.Split(f.ActionWhiteList[0], ",")
	}

	if len(f.ActionBlackList) == 1 {
		f.ActionBlackList = strings.Split(f.ActionBlackList[0], ",")
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

// PreChecker runs filters first, then calls underlying policy agent.
type PreChecker interface {
	// CheckHttpApi returns true if passes policy check. If not goes through
	// policy check, just return true.
	CheckHttpApi(string, string, *http.Request) (bool, error)

	CheckTask(db.Build, atc.TaskConfig) (bool, error)
}

// Checker should be implemented by policy agents.
type Checker interface {
	// Check returns true if passes policy check. If not goes through policy
	// check, just return true.
	Check(PolicyCheckInput) (bool, error)
}

type CheckerFactory interface {
	Description() string
	IsConfigured() bool
	NewChecker() (Checker, error)
}

var checkerFactories []CheckerFactory

func RegisterChecker(factory CheckerFactory) {
	checkerFactories = append(checkerFactories, factory)
}

func WireCheckers(group *flags.Group) {
	for _, factory := range checkerFactories {
		_, err := group.AddGroup(fmt.Sprintf("Policy Checker (%s)", factory.Description()), "", factory)
		if err != nil {
			panic(err)
		}
	}
}

var (
	clusterName    string
	clusterVersion string
)

func Initialize(logger lager.Logger, cluster string, version string, filter Filter) (PreChecker, error) {
	logger.Debug("policy-checker-initialize")

	clusterName = cluster
	clusterVersion = version

	var checkerDescriptions []string
	for _, factory := range checkerFactories {
		if factory.IsConfigured() {
			checkerDescriptions = append(checkerDescriptions, factory.Description())
		}
	}
	if len(checkerDescriptions) > 1 {
		return nil, fmt.Errorf("Multiple emitters configured: %s", strings.Join(checkerDescriptions, ", "))
	}

	for _, factory := range checkerFactories {
		if factory.IsConfigured() {
			realChecker, err := factory.NewChecker()
			if err != nil {
				return nil, err
			}
			return &preChecker{
				filter:      filter.normalize(),
				realChecker: realChecker,
			}, nil
		}
	}

	// No policy checker configured.
	return nil, nil
}

type preChecker struct {
	filter      Filter
	realChecker Checker
}

func (c *preChecker) CheckHttpApi(action string, user string, req *http.Request) (bool, error) {
	if c == nil {
		return true, nil
	}

	// Ignore self invoked API calls.
	if user == "system" {
		return true, nil
	}

	// Actions in black will not go through policy check.
	if c.actionInBlackList(action) {
		return true, nil
	}

	// Only actions with specified http method will go through policy check.
	// But actions in white list will always go through policy check.
	if !c.httpMethodShouldCheck(req) && !c.actionInWhiteList(action) {
		return true, nil
	}

	input := PolicyCheckInput{
		Service:        "concourse",
		ClusterName:    clusterName,
		ClusterVersion: clusterVersion,
		HttpMethod:     req.Method,
		Action:         action,
		User:           user,
		Team:           req.FormValue(":team_name"),
		Pipeline:       req.FormValue(":pipeline_name"),
	}

	switch req.Header.Get("Content-type") {
	case "application/json", "application/x-yaml":
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return false, err
		} else if body != nil && len(body) > 0 {
			err = yaml.Unmarshal(body, &input.Data)
			if err != nil {
				return false, err
			}

			req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		}
	}

	return c.realChecker.Check(input)
}

func (c *preChecker) httpMethodShouldCheck(req *http.Request) bool {
	return inArray(c.filter.HttpMethods, req.Method)
}

func (c *preChecker) actionInWhiteList(action string) bool {
	return inArray(c.filter.ActionWhiteList, action)
}

func (c *preChecker) actionInBlackList(action string) bool {
	return inArray(c.filter.ActionBlackList, action)
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

func (c *preChecker) CheckTask(build db.Build, config atc.TaskConfig) (bool, error) {
	if c == nil {
		return true, nil
	}

	// Actions in black will not go through policy check.
	if c.actionInBlackList(ActionRunTask) {
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

	return c.realChecker.Check(input)
}
