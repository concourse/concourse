package atc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"
)

type TaskConfig struct {
	// The platform the task must run on (e.g. linux, windows).
	Platform string `json:"platform,omitempty"`

	// Optional string specifying an image to use for the build. Depending on the
	// platform, this may or may not be required (e.g. Windows/OS X vs. Linux).
	RootfsURI string `json:"rootfs_uri,omitempty"`

	ImageResource *ImageResource `json:"image_resource,omitempty"`

	// Limits to set on the Task Container
	Limits *ContainerLimits `json:"container_limits,omitempty"`

	// Parameters to pass to the task via environment variables.
	Params TaskEnv `json:"params,omitempty"`

	// Script to execute.
	Run TaskRunConfig `json:"run,omitempty"`

	// The set of (logical, name-only) inputs required by the task.
	Inputs []TaskInputConfig `json:"inputs,omitempty"`

	// The set of (logical, name-only) outputs provided by the task.
	Outputs []TaskOutputConfig `json:"outputs,omitempty"`

	// Path to cached directory that will be shared between builds for the same task.
	Caches []TaskCacheConfig `json:"caches,omitempty"`
}

type ContainerLimits struct {
	CPU    *uint64 `json:"cpu,omitempty"`
	Memory *uint64 `json:"memory,omitempty"`
}

type ImageResource struct {
	Type   string `json:"type"`
	Source Source `json:"source"`

	Params  Params  `json:"params,omitempty"`
	Version Version `json:"version,omitempty"`
}

func (ir *ImageResource) ApplySourceDefaults(resourceTypes VersionedResourceTypes) {
	if ir == nil {
		return
	}

	parentType, found := resourceTypes.Lookup(ir.Type)
	if found {
		ir.Source = parentType.Defaults.Merge(ir.Source)
	} else {
		brtDefaults, found := FindBaseResourceTypeDefaults(ir.Type)
		if found {
			ir.Source = brtDefaults.Merge(ir.Source)
		}
	}
}

func NewTaskConfig(configBytes []byte) (TaskConfig, error) {
	var config TaskConfig
	err := yaml.UnmarshalStrict(configBytes, &config, yaml.DisallowUnknownFields)
	if err != nil {
		return TaskConfig{}, err
	}

	err = config.Validate()
	if err != nil {
		return TaskConfig{}, err
	}

	return config, nil
}

type TaskValidationError struct {
	Errors []string
}

func (err TaskValidationError) Error() string {
	return fmt.Sprintf("invalid task configuration:\n%s", strings.Join(err.Errors, "\n"))
}

func (config TaskConfig) Validate() error {
	var errors []string

	if config.Platform == "" {
		errors = append(errors, "missing 'platform'")
	}

	if config.Run.Path == "" {
		errors = append(errors, "missing path to executable to run")
	}

	errors = append(errors, config.validateInputContainsNames()...)
	errors = append(errors, config.validateOutputContainsNames()...)

	if len(errors) > 0 {
		return TaskValidationError{
			Errors: errors,
		}
	}

	return nil
}

func (config TaskConfig) validateOutputContainsNames() []string {
	var messages []string

	for i, output := range config.Outputs {
		if output.Name == "" {
			messages = append(messages, fmt.Sprintf("  output in position %d is missing a name", i))
		}
	}

	return messages
}

func (config TaskConfig) validateInputContainsNames() []string {
	messages := []string{}

	for i, input := range config.Inputs {
		if input.Name == "" {
			messages = append(messages, fmt.Sprintf("  input in position %d is missing a name", i))
		}
	}

	return messages
}

type TaskRunConfig struct {
	Path string   `json:"path"`
	Args []string `json:"args,omitempty"`
	Dir  string   `json:"dir,omitempty"`

	// The user that the task will run as (defaults to whatever the docker image specifies)
	User string `json:"user,omitempty"`
}

type TaskInputConfig struct {
	Name     string `json:"name"`
	Path     string `json:"path,omitempty"`
	Optional bool   `json:"optional,omitempty"`
}

type TaskOutputConfig struct {
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
}

type TaskCacheConfig struct {
	Path string `json:"path,omitempty"`
}

type TaskEnv map[string]string

func (te *TaskEnv) UnmarshalJSON(p []byte) error {
	raw := map[string]CoercedString{}
	err := json.Unmarshal(p, &raw)
	if err != nil {
		return err
	}

	m := map[string]string{}
	for k, v := range raw {
		m[k] = string(v)
	}

	*te = m

	return nil
}

func (te TaskEnv) Env() []string {
	env := make([]string, 0, len(te))

	for k, v := range te {
		env = append(env, k+"="+v)
	}

	return env
}

type CoercedString string

func (cs *CoercedString) UnmarshalJSON(p []byte) error {
	var raw interface{}
	dec := json.NewDecoder(bytes.NewReader(p))
	dec.UseNumber()
	err := dec.Decode(&raw)
	if err != nil {
		return err
	}

	if raw == nil {
		*cs = CoercedString("")
		return nil
	}
	switch v := raw.(type) {
	case string:
		*cs = CoercedString(v)

	case json.Number:
		*cs = CoercedString(v)

	default:
		j, err := json.Marshal(v)
		if err != nil {
			return err
		}

		*cs = CoercedString(j)
	}

	return nil
}
