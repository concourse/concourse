package atc

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const ConfigVersionHeader = "X-Concourse-Config-Version"
const DefaultPipelineName = "main"
const DefaultTeamName = "main"

type Tags []string

type ConfigResponse struct {
	Config    *Config   `json:"config"`
	Errors    []string  `json:"errors"`
	RawConfig RawConfig `json:"raw_config"`
}

type Config struct {
	Groups        GroupConfigs    `yaml:"groups" json:"groups" mapstructure:"groups"`
	Resources     ResourceConfigs `yaml:"resources" json:"resources" mapstructure:"resources"`
	ResourceTypes ResourceTypes   `yaml:"resource_types" json:"resource_types" mapstructure:"resource_types"`
	Jobs          JobConfigs      `yaml:"jobs" json:"jobs" mapstructure:"jobs"`
}

type RawConfig string

func (r RawConfig) String() string {
	return string(r)
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
	Name         string `yaml:"name" json:"name" mapstructure:"name"`
	WebhookToken string `yaml:"webhook_token,omitempty" json:"webhook_token" mapstructure:"webhook_token"`
	Type         string `yaml:"type" json:"type" mapstructure:"type"`
	Source       Source `yaml:"source" json:"source" mapstructure:"source"`
	CheckEvery   string `yaml:"check_every,omitempty" json:"check_every" mapstructure:"check_every"`
	Tags         Tags   `yaml:"tags,omitempty" json:"tags" mapstructure:"tags"`
}

type ResourceType struct {
	Name       string `yaml:"name" json:"name" mapstructure:"name"`
	Type       string `yaml:"type" json:"type" mapstructure:"type"`
	Source     Source `yaml:"source" json:"source" mapstructure:"source"`
	Privileged bool   `yaml:"privileged,omitempty" json:"privileged" mapstructure:"privileged"`
	Tags       Tags   `yaml:"tags,omitempty" json:"tags" mapstructure:"tags"`
	Params     Params `yaml:"params,omitempty" json:"params" mapstructure:"params"`
}

type ResourceTypes []ResourceType

func (types ResourceTypes) Lookup(name string) (ResourceType, bool) {
	for _, t := range types {
		if t.Name == name {
			return t, true
		}
	}

	return ResourceType{}, false
}

func (types ResourceTypes) Without(name string) ResourceTypes {
	newTypes := ResourceTypes{}
	for _, t := range types {
		if t.Name != name {
			newTypes = append(newTypes, t)
		}
	}

	return newTypes
}

type Hooks struct {
	Abort   *PlanConfig
	Failure *PlanConfig
	Ensure  *PlanConfig
	Success *PlanConfig
}

// A PlanSequence corresponds to a chain of Compose plan, with an implicit
// `on: [success]` after every Task plan.
type PlanSequence []PlanConfig

// A VersionConfig represents the choice to include every version of a
// resource, the latest version of a resource, or a pinned (specific) one.
type VersionConfig struct {
	Every  bool    `yaml:"every,omitempty" json:"every,omitempty"`
	Latest bool    `yaml:"latest,omitempty" json:"latest,omitempty"`
	Pinned Version `yaml:"pinned,omitempty" json:"pinned,omitempty"`
}

func (c *VersionConfig) UnmarshalJSON(version []byte) error {
	var data interface{}

	err := json.Unmarshal(version, &data)
	if err != nil {
		return err
	}

	switch actual := data.(type) {
	case string:
		c.Every = actual == "every"
		c.Latest = actual == "latest"
	case map[string]interface{}:
		version := Version{}

		for k, v := range actual {
			if s, ok := v.(string); ok {
				version[k] = strings.TrimSpace(s)
			}
		}

		c.Pinned = version
	default:
		return errors.New("unknown type for version")
	}

	return nil
}

func (c *VersionConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var data interface{}

	err := unmarshal(&data)
	if err != nil {
		return err
	}

	switch actual := data.(type) {
	case string:
		c.Every = actual == "every"
		c.Latest = actual == "latest"
	case map[interface{}]interface{}:
		version := Version{}

		for k, v := range actual {
			if ks, ok := k.(string); ok {
				if vs, ok := v.(string); ok {
					version[ks] = strings.TrimSpace(vs)
				}
			}
		}

		c.Pinned = version
	default:
		return errors.New("unknown type for version")
	}

	return nil
}

func (c *VersionConfig) MarshalYAML() (interface{}, error) {
	if c.Latest {
		return VersionLatest, nil
	}

	if c.Every {
		return VersionEvery, nil
	}

	if c.Pinned != nil {
		return c.Pinned, nil
	}

	return nil, nil
}

func (c *VersionConfig) MarshalJSON() ([]byte, error) {
	if c.Latest {
		return json.Marshal(VersionLatest)
	}

	if c.Every {
		return json.Marshal(VersionEvery)
	}

	if c.Pinned != nil {
		return json.Marshal(c.Pinned)
	}

	return json.Marshal("")
}

// A PlanConfig is a flattened set of configuration corresponding to
// a particular Plan, where Source and Version are populated lazily.
type PlanConfig struct {
	// makes the Plan conditional
	// conditions on which to perform a nested sequence

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

	// used to pass specific inputs/outputs as generic inputs/outputs in task config
	InputMapping  map[string]string `yaml:"input_mapping,omitempty" json:"input_mapping,omitempty" mapstructure:"input_mapping"`
	OutputMapping map[string]string `yaml:"output_mapping,omitempty" json:"output_mapping,omitempty" mapstructure:"output_mapping"`

	// used to specify an image artifact from a previous build to be used as the image for a subsequent task container
	ImageArtifactName string `yaml:"image,omitempty" json:"image,omitempty" mapstructure:"image"`

	// used by Put to specify params for the subsequent Get
	GetParams Params `yaml:"get_params,omitempty" json:"get_params,omitempty" mapstructure:"get_params"`

	// used by any step to specify which workers are eligible to run the step
	Tags Tags `yaml:"tags,omitempty" json:"tags,omitempty" mapstructure:"tags"`

	// used by any step to run something when the build is aborted during execution of the step
	Abort *PlanConfig `yaml:"on_abort,omitempty" json:"on_abort,omitempty" mapstructure:"on_abort"`

	// used by any step to run something when the step reports a failure
	Failure *PlanConfig `yaml:"on_failure,omitempty" json:"on_failure,omitempty" mapstructure:"on_failure"`

	// used on any step to always execute regardless of the step's completed state
	Ensure *PlanConfig `yaml:"ensure,omitempty" json:"ensure,omitempty" mapstructure:"ensure"`

	// used on any step to execute on successful completion of the step
	Success *PlanConfig `yaml:"on_success,omitempty" json:"on_success,omitempty" mapstructure:"on_success"`

	// used on any step to swallow failures and errors
	Try *PlanConfig `yaml:"try,omitempty" json:"try,omitempty" mapstructure:"try"`

	// used on any step to interrupt the step after a given duration
	Timeout string `yaml:"timeout,omitempty" json:"timeout,omitempty" mapstructure:"timeout"`

	// not present in yaml
	DependentGet string `yaml:"-" json:"-"`

	// repeat the step up to N times, until it works
	Attempts int `yaml:"attempts,omitempty" json:"attempts,omitempty" mapstructure:"attempts"`

	Version *VersionConfig `yaml:"version,omitempty" json:"version,omitempty" mapstructure:"version"`
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

func (config PlanConfig) ResourceName() string {
	resourceName := config.Resource
	if resourceName != "" {
		return resourceName
	}

	resourceName = config.Get
	if resourceName != "" {
		return resourceName
	}

	resourceName = config.Put
	if resourceName != "" {
		return resourceName
	}

	panic("no resource name!")
}

func (config PlanConfig) Hooks() Hooks {
	return Hooks{Abort: config.Abort, Failure: config.Failure, Ensure: config.Ensure, Success: config.Success}
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
