package atc

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

const ConfigVersionHeader = "X-Concourse-Config-Version"
const DefaultPipelineName = "main"
const DefaultTeamName = "main"

type Tags []string

type Config struct {
	Groups        GroupConfigs    `json:"groups,omitempty"`
	Resources     ResourceConfigs `json:"resources,omitempty"`
	ResourceTypes ResourceTypes   `json:"resource_types,omitempty"`
	Jobs          JobConfigs      `json:"jobs,omitempty"`
}

type GroupConfig struct {
	Name      string   `json:"name"`
	Jobs      []string `json:"jobs,omitempty"`
	Resources []string `json:"resources,omitempty"`
}

type GroupConfigs []GroupConfig

func (groups GroupConfigs) Lookup(name string) (GroupConfig, int, bool) {
	for index, group := range groups {
		if group.Name == name {
			return group, index, true
		}
	}

	return GroupConfig{}, -1, false
}

type ResourceConfig struct {
	Name         string  `json:"name"`
	Public       bool    `json:"public,omitempty"`
	WebhookToken string  `json:"webhook_token,omitempty"`
	Type         string  `json:"type"`
	Source       Source  `json:"source"`
	CheckEvery   string  `json:"check_every,omitempty"`
	CheckTimeout string  `json:"check_timeout,omitempty"`
	Tags         Tags    `json:"tags,omitempty"`
	Version      Version `json:"version,omitempty"`
	Icon         string  `json:"icon,omitempty"`
}

type ResourceType struct {
	Name                 string `json:"name"`
	Type                 string `json:"type"`
	Source               Source `json:"source"`
	Privileged           bool   `json:"privileged,omitempty"`
	CheckEvery           string `json:"check_every,omitempty"`
	Tags                 Tags   `json:"tags,omitempty"`
	Params               Params `json:"params,omitempty"`
	CheckSetupError      string `json:"check_setup_error,omitempty"`
	CheckError           string `json:"check_error,omitempty"`
	UniqueVersionHistory bool   `json:"unique_version_history,omitempty"`
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
	Error   *PlanConfig
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
	Every  bool
	Latest bool
	Pinned Version
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

const VersionLatest = "latest"
const VersionEvery = "every"

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

// A InputsConfig represents the choice to include every artifact within the
// job as an input to the put step or specific ones.
type InputsConfig struct {
	All       bool
	Specified []string
}

func (c *InputsConfig) UnmarshalJSON(inputs []byte) error {
	var data interface{}

	err := json.Unmarshal(inputs, &data)
	if err != nil {
		return err
	}

	switch actual := data.(type) {
	case string:
		c.All = actual == "all"
	case []interface{}:
		inputs := []string{}

		for _, v := range actual {
			str, ok := v.(string)
			if !ok {
				return fmt.Errorf("non-string put input: %v", v)
			}

			inputs = append(inputs, strings.TrimSpace(str))
		}

		c.Specified = inputs
	default:
		return errors.New("unknown type for put inputs")
	}

	return nil
}

const InputsAll = "all"

func (c InputsConfig) MarshalJSON() ([]byte, error) {
	if c.All {
		return json.Marshal(InputsAll)
	}

	if c.Specified != nil {
		return json.Marshal(c.Specified)
	}

	return json.Marshal("")
}

type InParallelConfig struct {
	Steps    PlanSequence `json:"steps,omitempty"`
	Limit    int          `json:"limit,omitempty"`
	FailFast bool         `json:"fail_fast,omitempty"`
}

func (c *InParallelConfig) UnmarshalJSON(payload []byte) error {
	var data interface{}
	err := json.Unmarshal(payload, &data)
	if err != nil {
		return err
	}

	switch actual := data.(type) {
	case []interface{}:
		if err := json.Unmarshal(payload, &c.Steps); err != nil {
			return fmt.Errorf("failed to unmarshal parallel steps: %s", err)
		}
	case map[string]interface{}:
		// Used to avoid infinite recursion when unmarshalling this variant.
		type target InParallelConfig

		var t target
		if err := json.Unmarshal(payload, &t); err != nil {
			return fmt.Errorf("failed to unmarshal parallel config: %s", err)
		}

		c.Steps, c.Limit, c.FailFast = t.Steps, t.Limit, t.FailFast
	default:
		return fmt.Errorf("wrong type for parallel config: %v", actual)
	}

	return nil
}

// A PlanConfig is a flattened set of configuration corresponding to
// a particular Plan, where Source and Version are populated lazily.
type PlanConfig struct {
	// makes the Plan conditional
	// conditions on which to perform a nested sequence

	// compose a nested sequence of plans
	// name of the nested 'do'
	RawName string `json:"name,omitempty"`

	// a nested chain of steps to run
	Do *PlanSequence `json:"do,omitempty"`

	// corresponds to an Aggregate plan, keyed by the name of each sub-plan
	Aggregate *PlanSequence `json:"aggregate,omitempty"`

	// a nested chain of steps to run in parallel
	InParallel *InParallelConfig `json:"in_parallel,omitempty"`

	// corresponds to Get and Put resource plans, respectively
	// name of 'input', e.g. bosh-stemcell
	Get string `json:"get,omitempty"`
	// jobs that this resource must have made it through
	Passed []string `json:"passed,omitempty"`
	// whether to trigger based on this resource changing
	Trigger bool `json:"trigger,omitempty"`

	// name of 'output', e.g. rootfs-tarball
	Put string `json:"put,omitempty"`

	// corresponding resource config, e.g. aws-stemcell
	Resource string `json:"resource,omitempty"`

	// inputs to a put step either a list (e.g. [artifact-1, aritfact-2]) or all (e.g. all)
	Inputs *InputsConfig `json:"inputs,omitempty"`

	// corresponds to a Task plan
	// name of 'task', e.g. unit, go1.3, go1.4
	Task string `json:"task,omitempty"`
	// run task privileged
	Privileged bool `json:"privileged,omitempty"`
	// task config path, e.g. foo/build.yml
	TaskConfigPath string `json:"file,omitempty"`
	// task variables, if task is specified as external file via TaskConfigPath
	TaskVars Params `json:"vars,omitempty"`
	// inlined task config
	TaskConfig *TaskConfig `json:"config,omitempty"`

	// used by Get and Put for specifying params to the resource
	// used by Task for passing params to external task config
	Params Params `json:"params,omitempty"`

	// used to pass specific inputs/outputs as generic inputs/outputs in task config
	InputMapping  map[string]string `json:"input_mapping,omitempty"`
	OutputMapping map[string]string `json:"output_mapping,omitempty"`

	// used to specify an image artifact from a previous build to be used as the image for a subsequent task container
	ImageArtifactName string `json:"image,omitempty"`

	// used by Put to specify params for the subsequent Get
	GetParams Params `json:"get_params,omitempty"`

	// used by any step to specify which workers are eligible to run the step
	Tags Tags `json:"tags,omitempty"`

	// used by any step to run something when the build is aborted during execution of the step
	Abort *PlanConfig `json:"on_abort,omitempty"`

	// used by any step to run something when the build errors during execution of the step
	Error *PlanConfig `json:"on_error,omitempty"`

	// used by any step to run something when the step reports a failure
	Failure *PlanConfig `json:"on_failure,omitempty"`

	// used on any step to always execute regardless of the step's completed state
	Ensure *PlanConfig `json:"ensure,omitempty"`

	// used on any step to execute on successful completion of the step
	Success *PlanConfig `json:"on_success,omitempty"`

	// used on any step to swallow failures and errors
	Try *PlanConfig `json:"try,omitempty"`

	// used on any step to interrupt the step after a given duration
	Timeout string `json:"timeout,omitempty"`

	// not present in yaml
	DependentGet string `json:"-" json:"-"`

	// repeat the step up to N times, until it works
	Attempts int `json:"attempts,omitempty"`

	Version *VersionConfig `json:"version,omitempty"`
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
	return Hooks{Abort: config.Abort, Error: config.Error, Failure: config.Failure, Ensure: config.Ensure, Success: config.Success}
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

func DefaultTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,

		// https://wiki.mozilla.org/Security/Server_Side_TLS#Modern_compatibility
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.CurveP384,
			tls.CurveP521,
		},

		// Security team recommends a very restricted set of cipher suites
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},

		PreferServerCipherSuites: true,
		NextProtos:               []string{"h2"},
	}
}

func DefaultSSHConfig() ssh.Config {
	return ssh.Config{
		// use the defaults prefered by go, see https://github.com/golang/crypto/blob/master/ssh/common.go
		Ciphers: nil,

		// CIS recommends a certain set of MAC algorithms to be used in SSH connections. This restricts the set from a more permissive set used by default by Go.
		// See https://infosec.mozilla.org/guidelines/openssh.html and https://www.cisecurity.org/cis-benchmarks/
		MACs: []string{
			"hmac-sha2-256-etm@openssh.com",
			"hmac-sha2-256",
		},

		//[KEX Recommendations for SSH IETF](https://tools.ietf.org/html/draft-ietf-curdle-ssh-kex-sha2-10#section-4)
		//[Mozilla Openssh Reference](https://infosec.mozilla.org/guidelines/openssh.html)
		KeyExchanges: []string{
			"ecdh-sha2-nistp256",
			"ecdh-sha2-nistp384",
			"ecdh-sha2-nistp521",
			"curve25519-sha256@libssh.org",
		},
	}
}
