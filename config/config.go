package config

import (
	"fmt"
	"time"

	"github.com/concourse/turbine/api/builds"
)

type Config struct {
	Groups    Groups    `yaml:"groups"`
	Resources Resources `yaml:"resources"`
	Jobs      Jobs      `yaml:"jobs"`
}

type Group struct {
	Name      string   `yaml:"name"`
	Jobs      []string `yaml:"jobs"`
	Resources []string `yaml:"resources"`
}

type Groups []Group

type Resource struct {
	Name string `yaml:"name"`

	Type   string `yaml:"type"`
	Source Source `yaml:"source"`
}

type Source map[string]interface{}

type Job struct {
	Name string `yaml:"name"`

	Public bool `yaml:"public"`

	BuildConfigPath string        `yaml:"build"`
	BuildConfig     builds.Config `yaml:"config"`

	Privileged bool `yaml:"privileged"`

	Serial bool `yaml:"serial"`

	Inputs  []Input  `yaml:"inputs"`
	Outputs []Output `yaml:"outputs"`
}

type Input struct {
	Name      string   `yaml:"name"`
	Resource  string   `yaml:"resource"`
	Params    Params   `yaml:"params"`
	Passed    []string `yaml:"passed"`
	DontCheck bool     `yaml:"dont_check"`
}

type Output struct {
	Resource string `yaml:"resource"`
	Params   Params `yaml:"params"`

	// e.g. [success, failure]; default [success]
	On []OutputCondition `yaml:"on"`
}

type OutputCondition string

func (c *OutputCondition) UnmarshalYAML(tag string, value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("invalid output condition: %#v (must be success/failure)", value)
	}

	switch builds.OutputCondition(str) {
	case builds.OutputConditionSuccess, builds.OutputConditionFailure:
		*c = OutputCondition(str)
	default:
		return fmt.Errorf("unknown output condition: %s (must be success/failure)", str)
	}

	return nil
}

type Duration time.Duration

func (d *Duration) UnmarshalYAML(tag string, value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("invalid duration: %#v", value)
	}

	duration, err := time.ParseDuration(str)
	if err != nil {
		return err
	}

	*d = Duration(duration)

	return nil
}

type Params map[string]interface{}

type Resources []Resource

func (resources Resources) Lookup(name string) (Resource, bool) {
	for _, resource := range resources {
		if resource.Name == name {
			return resource, true
		}
	}

	return Resource{}, false
}

type Jobs []Job

func (jobs Jobs) Lookup(name string) (Job, bool) {
	for _, job := range jobs {
		if job.Name == name {
			return job, true
		}
	}

	return Job{}, false
}
