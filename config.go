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
	Jobs      []string `yaml:"jobs,omitempty" json:"jobs,omitempty"`
	Resources []string `yaml:"resources,omitempty" json:"resources,omitempty"`
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

	Public bool `yaml:"public,omitempty" json:"public,omitempty"`

	BuildConfigPath string         `yaml:"build,omitempty" json:"build,omitempty"`
	BuildConfig     turbine.Config `yaml:"config,omitempty" json:"config,omitempty"`

	Privileged bool `yaml:"privileged,omitempty" json:"privileged,omitempty"`

	Serial bool `yaml:"serial,omitempty" json:"serial,omitempty"`

	Inputs  []InputConfig  `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	Outputs []OutputConfig `yaml:"outputs,omitempty" json:"outputs,omitempty"`
}

type InputConfig struct {
	RawName    string   `yaml:"name,omitempty" json:"name,omitempty"`
	Resource   string   `yaml:"resource" json:"resource"`
	Params     Params   `yaml:"params,omitempty" json:"params,omitempty"`
	Passed     []string `yaml:"passed,omitempty" json:"passed,omitempty"`
	RawTrigger *bool    `yaml:"trigger" json:"trigger"`
	Hidden     bool     `yaml:"hidden" json:"hidden,omitempty"`
}

func (config InputConfig) Name() string {
	if len(config.RawName) > 0 {
		return config.RawName
	}

	return config.Resource
}

func (config InputConfig) Trigger() bool {
	return config.RawTrigger == nil || *config.RawTrigger
}

type OutputConfig struct {
	Resource string `yaml:"resource" json:"resource"`
	Params   Params `yaml:"params,omitempty" json:"params,omitempty"`

	// e.g. [success, failure]; default [success]
	RawPerformOn []OutputCondition `yaml:"perform_on,omitempty" json:"perform_on,omitempty"`
}

func (config OutputConfig) PerformOn() []OutputCondition {
	if config.RawPerformOn == nil { // NOT len(0)
		return []OutputCondition{"success"}
	}

	return config.RawPerformOn
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

func (config Config) JobIsPublic(jobName string) (bool, error) {
	job, found := config.Jobs.Lookup(jobName)
	if !found {
		return false, fmt.Errorf("cannot find job with job name '%s'", jobName)
	}

	return job.Public, nil
}
