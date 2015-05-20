package atc

import (
	"errors"
	"fmt"
	"time"
)

const ConfigVersionHeader = "X-Concourse-Config-Version"
const DefaultPipelineName = "main"

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
	Name         string   `yaml:"name" json:"name" mapstructure:"name"`
	Public       bool     `yaml:"public,omitempty" json:"public,omitempty" mapstructure:"public"`
	Serial       bool     `yaml:"serial,omitempty" json:"serial,omitempty" mapstructure:"serial"`
	SerialGroups []string `yaml:"serial_groups,omitempty" json:"serial_groups,omitempty" mapstructure:"serial_groups"`

	Privileged     bool        `yaml:"privileged,omitempty" json:"privileged,omitempty" mapstructure:"privileged"`
	TaskConfigPath string      `yaml:"build,omitempty" json:"build,omitempty" mapstructure:"build"`
	TaskConfig     *TaskConfig `yaml:"config,omitempty" json:"config,omitempty" mapstructure:"config"`

	InputConfigs  []JobInputConfig  `yaml:"inputs,omitempty" json:"inputs,omitempty" mapstructure:"inputs"`
	OutputConfigs []JobOutputConfig `yaml:"outputs,omitempty" json:"outputs,omitempty" mapstructure:"outputs"`

	Plan PlanSequence `yaml:"plan,omitempty" json:"plan,omitempty" mapstructure:"plan"`
}

func (config JobConfig) IsSerial() bool {
	return config.Serial || len(config.SerialGroups) > 0
}

func (config JobConfig) GetSerialGroups() []string {
	if len(config.SerialGroups) > 0 {
		return config.SerialGroups
	}

	if config.IsSerial() {
		return []string{config.Name}
	}

	return []string{}
}

func (config JobConfig) Inputs() []JobInput {
	if config.InputConfigs != nil {
		var inputs []JobInput

		for _, config := range config.InputConfigs {
			inputs = append(inputs, JobInput{
				Name:     config.Name(),
				Resource: config.Resource,
				Passed:   config.Passed,
				Trigger:  config.Trigger,
			})
		}

		return inputs
	}

	return collectInputs(PlanConfig{Do: &config.Plan})
}

func (config JobConfig) Outputs() []JobOutput {
	if config.OutputConfigs != nil {
		var outputs []JobOutput

		for _, config := range config.OutputConfigs {
			outputs = append(outputs, JobOutput{
				Name:     config.Resource,
				Resource: config.Resource,
			})
		}

		return outputs
	}

	return collectOutputs(PlanConfig{Do: &config.Plan})
}

// A PlanSequence corresponds to a chain of Compose plan, with an implicit
// `on: [success]` after every Task plan.
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

	// a nested chain of steps to run
	Do *PlanSequence `yaml:"do,omitempty" json:"do,omitempty" mapstructure:"do"`

	// corresponds to an Aggregate plan, keyed by the name of each sub-plan
	Aggregate *PlanSequence `yaml:"aggregate,omitempty" json:"aggregate,omitempty" mapstructure:"aggregate"`

	// corresponds to Get and Put resource plans, respectively
	// name of 'input', e.g. bosh-stemcell
	Get string `yaml:"get,omitempty" json:"get,omitempty" mapstructure:"get"`
	// jobs that this resource must have made it through
	Passed []string `yaml:"passed,omitempty" json:"passed,omitempty" mapstructure:"passed"`
	// whether to trigger based on this resource changing
	Trigger bool `yaml:"trigger,omitempty" json:"trigger,omitempty" mapstructure:"trigger"`

	// name of 'output', e.g. rootfs-tarball
	Put string `yaml:"put,omitempty" json:"put,omitempty" mapstructure:"put"`

	// corresponding resource config, e.g. aws-stemcell
	Resource string `yaml:"resource,omitempty" json:"resource,omitempty" mapstructure:"resource"`

	// corresponds to a Task plan
	// name of 'task', e.g. unit, go1.3, go1.4
	Task string `yaml:"task,omitempty" json:"task,omitempty" mapstructure:"task"`
	// run task privileged
	Privileged bool `yaml:"privileged,omitempty" json:"privileged,omitempty" mapstructure:"privileged"`
	// task config path, e.g. foo/build.yml
	TaskConfigPath string `yaml:"file,omitempty" json:"file,omitempty" mapstructure:"file"`
	// inlined task config
	TaskConfig *TaskConfig `yaml:"config,omitempty" json:"config,omitempty" mapstructure:"config"`

	// used by Get and Put for specifying params to the resource
	Params Params `yaml:"params,omitempty" json:"params,omitempty" mapstructure:"params"`

	// used by Put to specify params for the subsequent Get
	GetParams Params `yaml:"get_params,omitempty" json:"get_params,omitempty" mapstructure:"get_params"`
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

	if config.Task != "" {
		return config.Task
	}

	return ""
}

type JobInputConfig struct {
	RawName  string   `yaml:"name,omitempty" json:"name,omitempty" mapstructure:"name"`
	Resource string   `yaml:"resource" json:"resource" mapstructure:"resource"`
	Params   Params   `yaml:"params,omitempty" json:"params,omitempty" mapstructure:"params"`
	Passed   []string `yaml:"passed,omitempty" json:"passed,omitempty" mapstructure:"passed"`
	Trigger  bool     `yaml:"trigger" json:"trigger" mapstructure:"trigger"`
}

func (config JobInputConfig) Name() string {
	if len(config.RawName) > 0 {
		return config.RawName
	}

	return config.Resource
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

type Conditions []Condition

func (cs Conditions) SatisfiedBy(successful bool) bool {
	for _, status := range cs {
		if (status == ConditionSuccess && successful) ||
			(status == ConditionFailure && !successful) {
			return true
		}
	}

	return false
}

type Condition string

const (
	ConditionSuccess Condition = "success"
	ConditionFailure Condition = "failure"
)

func (c *Condition) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var str string
	if err := unmarshal(&str); err != nil {
		return fmt.Errorf("invalid condition: %s", err)
	}

	switch Condition(str) {
	case ConditionSuccess, ConditionFailure:
		*c = Condition(str)
	default:
		return fmt.Errorf("unknown condition: %s (must be success/failure)", str)
	}

	return nil
}

type Duration time.Duration

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var num int64
	if err := unmarshal(&num); err == nil {
		*d = Duration(num)
		return nil
	}

	var str string
	if err := unmarshal(&str); err != nil {
		return errors.New("invalid duration; must be string or number")
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

func collectInputs(plan PlanConfig) []JobInput {
	if plan.Do != nil {
		var inputs []JobInput

		for _, p := range *plan.Do {
			inputs = append(inputs, collectInputs(p)...)
		}

		return inputs
	}

	if plan.Get != "" && plan.Put == "" {
		get := plan.Get

		resource := get
		if plan.Resource != "" {
			resource = plan.Resource
		}

		return []JobInput{
			{
				Name:     get,
				Resource: resource,
				Passed:   plan.Passed,
				Trigger:  plan.Trigger,
			},
		}
	}

	if plan.Aggregate != nil {
		var inputs []JobInput

		for _, p := range *plan.Aggregate {
			inputs = append(inputs, collectInputs(p)...)
		}

		return inputs
	}

	return []JobInput{}
}

func collectOutputs(plan PlanConfig) []JobOutput {
	if plan.Do != nil {
		var outputs []JobOutput

		for _, p := range *plan.Do {
			outputs = append(outputs, collectOutputs(p)...)
		}

		return outputs
	}

	if plan.Put != "" {
		put := plan.Put

		resource := put
		if plan.Resource != "" {
			resource = plan.Resource
		}

		return []JobOutput{
			{
				Name:     put,
				Resource: resource,
			},
		}
	}

	if plan.Aggregate != nil {
		var outputs []JobOutput

		for _, p := range *plan.Aggregate {
			outputs = append(outputs, collectOutputs(p)...)
		}

		return outputs
	}

	return []JobOutput{}
}
