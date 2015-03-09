package atc

import (
	"fmt"
	"time"
)

const ConfigIDHeader = "X-Concourse-Config-ID"

type Source map[string]interface{}
type Params map[string]interface{}
type Version map[string]interface{}

type Config struct {
	Groups    GroupConfigs    `yaml:"groups" json:"groups" mapstructure:"groups"`
	Resources ResourceConfigs `yaml:"resources" json:"resources" mapstructure:"resources"`
	Jobs      JobConfigs      `yaml:"jobs" json:"jobs" mapstructure:"jobs"`
}

type GroupConfig struct {
	Name      string   `yaml:"name" json:"name" mapstructure:"name"`
	Jobs      []string `yaml:"jobs,omitempty" json:"jobs,omitempty" mapstructure:"jobs"`
	Resources []string `yaml:"resources,omitempty" json:"resources,omitempty" mapstructure:"resources"`
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
	Name string `yaml:"name" json:"name" mapstructure:"name"`

	Type   string `yaml:"type" json:"type" mapstructure:"type"`
	Source Source `yaml:"source" json:"source" mapstructure:"source"`
}

type JobConfig struct {
	Name   string `yaml:"name" json:"name" mapstructure:"name"`
	Public bool   `yaml:"public,omitempty" json:"public,omitempty" mapstructure:"public"`
	Serial bool   `yaml:"serial,omitempty" json:"serial,omitempty" mapstructure:"serial"`

	Privileged      bool         `yaml:"privileged,omitempty" json:"privileged,omitempty" mapstructure:"privileged"`
	BuildConfigPath string       `yaml:"build,omitempty" json:"build,omitempty" mapstructure:"build"`
	BuildConfig     *BuildConfig `yaml:"config,omitempty" json:"config,omitempty" mapstructure:"config"`

	Inputs  []JobInputConfig  `yaml:"inputs,omitempty" json:"inputs,omitempty" mapstructure:"inputs"`
	Outputs []JobOutputConfig `yaml:"outputs,omitempty" json:"outputs,omitempty" mapstructure:"outputs"`

	Plan PlanSequence `yaml:"plan,omitempty" json:"plan,omitempty" mapstructure:"plan"`
}

// A PlanSequence corresponds to a chain of Compose plan, with an implicit
// `on: [success]` after every Execute plan.
type PlanSequence []PlanConfig

// A PlanConfig is a flattened set of configuration corresponding to
// a particular Plan, where Source and Version are populated lazily.
type PlanConfig struct {
	// makes the Plan conditional
	// conditions on which to perform a nested sequence
	Conditions *Conditions `yaml:"conditions,omitempty" json:"conditions,omitempty" mapstructure:"conditions"`

	// compose a nested sequence of plans
	// name of the nested 'do'
	RawName string `yaml:"name,omitempty" json:"name,omitempty" mapstructure:"name"`
	// sequence to execute if conditions are met
	Do *PlanSequence `yaml:"do,omitempty" json:"do,omitempty" mapstructure:"do"`

	// corresponds to an Aggregate plan, keyed by the name of each sub-plan
	Aggregate *PlanSequence `yaml:"aggregate,omitempty" json:"aggregate,omitempty" mapstructure:"aggregate"`

	// corresponds to Get and Put resource plans, respectively
	// name of 'input', e.g. bosh-stemcell
	Get string `yaml:"get,omitempty" json:"get,omitempty" mapstructure:"get"`
	// name of 'output', e.g. rootfs-tarball
	Put string `yaml:"put,omitempty" json:"put,omitempty" mapstructure:"put"`
	// corresponding resource config, e.g. aws-stemcell
	Resource string `yaml:"resource,omitempty" json:"resource,omitempty" mapstructure:"resource"`

	// corresponds to an Execute plan
	// name of 'execute', e.g. unit, go1.3, go1.4
	Execute string `yaml:"execute,omitempty" json:"execute,omitempty" mapstructure:"execute"`
	// run build privileged
	Privileged bool `yaml:"privileged,omitempty" json:"privileged,omitempty" mapstructure:"privileged"`
	// build config path, e.g. foo/build.yml
	BuildConfigPath string `yaml:"build,omitempty" json:"build,omitempty" mapstructure:"build"`
	// inlined build config
	BuildConfig *BuildConfig `yaml:"config,omitempty" json:"config,omitempty" mapstructure:"config"`

	// used by Get, Put, and Execute for specifying params to resource or
	// build
	//
	// for Execute, it's shorthand for BuildConfig.Params
	Params Params `yaml:"params,omitempty" json:"params,omitempty" mapstructure:"params"`
}

func (config PlanConfig) Name() string {
	if config.RawName != "" {
		return config.RawName
	}

	if config.Get != "" {
		return config.Get
	}

	if config.Put != "" {
		return config.Put
	}

	if config.Execute != "" {
		return config.Execute
	}

	return ""
}

type JobInputConfig struct {
	RawName    string   `yaml:"name,omitempty" json:"name,omitempty" mapstructure:"name"`
	Resource   string   `yaml:"resource" json:"resource" mapstructure:"resource"`
	Params     Params   `yaml:"params,omitempty" json:"params,omitempty" mapstructure:"params"`
	Passed     []string `yaml:"passed,omitempty" json:"passed,omitempty" mapstructure:"passed"`
	RawTrigger *bool    `yaml:"trigger" json:"trigger" mapstructure:"trigger"`
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
	Resource string `yaml:"resource" json:"resource" mapstructure:"resource"`
	Params   Params `yaml:"params,omitempty" json:"params,omitempty" mapstructure:"params"`

	// e.g. [success, failure]; default [success]
	RawPerformOn []Condition `yaml:"perform_on,omitempty" json:"perform_on,omitempty" mapstructure:"perform_on"`
}

func (config JobOutputConfig) PerformOn() []Condition {
	if config.RawPerformOn == nil { // NOT len(0)
		return []Condition{"success"}
	}

	return config.RawPerformOn
}

type Condition string

const (
	ConditionSuccess Condition = "success"
	ConditionFailure Condition = "failure"
)

func (c *Condition) UnmarshalYAML(tag string, value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("invalid output condition: %#v (must be success/failure)", value)
	}

	switch Condition(str) {
	case ConditionSuccess, ConditionFailure:
		*c = Condition(str)
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
