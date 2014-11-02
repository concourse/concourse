package atc

import (
	"fmt"
	"time"

	"github.com/concourse/turbine"
)

type Config struct {
	Groups    GroupConfigs    `yaml:"groups" json:"groups"`
	Resources ResourceConfigs `yaml:"resources" json:"resources"`
	Jobs      JobConfigs      `yaml:"jobs" json:"jobs"`
}

type GroupConfig struct {
	Name      string   `yaml:"name" json:"name"`
	Jobs      []string `yaml:"jobs" json:"jobs"`
	Resources []string `yaml:"resources" json:"resources"`
}

type GroupConfigs []GroupConfig

type ResourceConfig struct {
	Name string `yaml:"name" json:"name"`

	Type   string `yaml:"type" json:"type"`
	Source Source `yaml:"source" json:"source"`
}

type Source map[string]interface{}

type JobConfig struct {
	Name string `yaml:"name" json:"name"`

	Public bool `yaml:"public" json:"public"`

	BuildConfigPath string         `yaml:"build" json:"build"`
	BuildConfig     turbine.Config `yaml:"config" json:"config"`

	Privileged bool `yaml:"privileged" json:"privileged"`

	Serial bool `yaml:"serial" json:"serial"`

	Inputs  []InputConfig  `yaml:"inputs" json:"inputs"`
	Outputs []OutputConfig `yaml:"outputs" json:"outputs"`
}

type InputConfig struct {
	Name     string   `yaml:"name" json:"name"`
	Resource string   `yaml:"resource" json:"resource"`
	Params   Params   `yaml:"params" json:"params"`
	Passed   []string `yaml:"passed" json:"passed"`
	Trigger  *bool    `yaml:"trigger" json:"trigger"`
}

type OutputConfig struct {
	Resource string `yaml:"resource" json:"resource"`
	Params   Params `yaml:"params" json:"params"`

	// e.g. [success, failure]; default [success]
	PerformOn []OutputCondition `yaml:"perform_on" json:"perform_on"`
}

type OutputCondition string

func (c *OutputCondition) UnmarshalYAML(tag string, value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("invalid output condition: %#v (must be success/failure)", value)
	}

	switch turbine.OutputCondition(str) {
	case turbine.OutputConditionSuccess, turbine.OutputConditionFailure:
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

type ResourceConfigs []ResourceConfig

func (resources ResourceConfigs) Lookup(name string) (ResourceConfig, bool) {
	for _, resource := range resources {
		if resource.Name == name {
			return resource, true
		}
	}

	return ResourceConfig{}, false
}

type JobConfigs []JobConfig

func (jobs JobConfigs) Lookup(name string) (JobConfig, bool) {
	for _, job := range jobs {
		if job.Name == name {
			return job, true
		}
	}

	return JobConfig{}, false
}
