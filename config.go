package atc

import (
	"fmt"
	"time"
)

type Source map[string]interface{}
type Params map[string]interface{}
type Version map[string]interface{}

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

func (groups GroupConfigs) Lookup(name string) (GroupConfig, bool) {
	for _, group := range groups {
		if group.Name == name {
			return group, true
		}
	}

	return GroupConfig{}, false
}

type ResourceConfig struct {
	Name string `yaml:"name" json:"name"`

	Type   string `yaml:"type" json:"type"`
	Source Source `yaml:"source" json:"source"`
}

type JobConfig struct {
	Name string `yaml:"name" json:"name"`

	Public bool `yaml:"public,omitempty" json:"public,omitempty"`

	BuildConfigPath string       `yaml:"build,omitempty" json:"build,omitempty"`
	BuildConfig     *BuildConfig `yaml:"config,omitempty" json:"config,omitempty"`

	Privileged bool `yaml:"privileged,omitempty" json:"privileged,omitempty"`

	Serial bool `yaml:"serial,omitempty" json:"serial,omitempty"`

	Inputs  []JobInputConfig  `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	Outputs []JobOutputConfig `yaml:"outputs,omitempty" json:"outputs,omitempty"`
}

type JobInputConfig struct {
	RawName    string   `yaml:"name,omitempty" json:"name,omitempty"`
	Resource   string   `yaml:"resource" json:"resource"`
	Params     Params   `yaml:"params,omitempty" json:"params,omitempty"`
	Passed     []string `yaml:"passed,omitempty" json:"passed,omitempty"`
	RawTrigger *bool    `yaml:"trigger" json:"trigger"`
}

func (config JobInputConfig) Name() string {
	if len(config.RawName) > 0 {
		return config.RawName
	}

	return config.Resource
}

func (config JobInputConfig) Trigger() bool {
	return config.RawTrigger == nil || *config.RawTrigger
}

type JobOutputConfig struct {
	Resource string `yaml:"resource" json:"resource"`
	Params   Params `yaml:"params,omitempty" json:"params,omitempty"`

	// e.g. [success, failure]; default [success]
	RawPerformOn []OutputCondition `yaml:"perform_on,omitempty" json:"perform_on,omitempty"`
}

func (config JobOutputConfig) PerformOn() []OutputCondition {
	if config.RawPerformOn == nil { // NOT len(0)
		return []OutputCondition{"success"}
	}

	return config.RawPerformOn
}

type OutputCondition string

const (
	OutputConditionSuccess OutputCondition = "success"
	OutputConditionFailure OutputCondition = "failure"
)

func (c *OutputCondition) UnmarshalYAML(tag string, value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("invalid output condition: %#v (must be success/failure)", value)
	}

	switch OutputCondition(str) {
	case OutputConditionSuccess, OutputConditionFailure:
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
