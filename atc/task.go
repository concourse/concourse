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

	// UnknownFields holds any fields that were present in the config but not
	// recognized. This is used for validation to report typos like "ouputs"
	// instead of "outputs".
	UnknownFields map[string]*json.RawMessage `json:"-"`
}

type ImageResource struct {
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Source  Source  `json:"source"`
	Version Version `json:"version,omitempty"`
	Params  Params  `json:"params,omitempty"`
	Tags    Tags    `json:"tags,omitempty"`
}

func (ir *ImageResource) ApplySourceDefaults(resourceTypes ResourceTypes) {
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

// UnmarshalJSON implements custom unmarshaling for TaskConfig to detect
// unknown fields (like typos such as "ouputs" instead of "outputs").
func (config *TaskConfig) UnmarshalJSON(data []byte) error {
	// First, unmarshal into a map to capture all fields
	var rawConfig map[string]*json.RawMessage
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		return err
	}

	// Use a type alias to avoid infinite recursion
	type taskConfigAlias TaskConfig
	var alias taskConfigAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	*config = TaskConfig(alias)

	deleteKnownFields(rawConfig, config)
	if len(rawConfig) != 0 {
		config.UnknownFields = rawConfig
	}

	return nil
}

func NewTaskConfig(configBytes []byte) (TaskConfig, error) {
	var config TaskConfig
	// Note: yaml.UnmarshalStrict's DisallowUnknownFields doesn't work with
	// custom UnmarshalJSON methods, so we rely on UnknownFields being populated
	// by our UnmarshalJSON and check it explicitly below.
	err := yaml.Unmarshal(configBytes, &config)
	if err != nil {
		return TaskConfig{}, err
	}

	// Check for unknown fields (e.g., typos like "ouputs" instead of "outputs")
	if len(config.UnknownFields) > 0 {
		var fieldNames []string
		for field := range config.UnknownFields {
			fieldNames = append(fieldNames, field)
		}
		return TaskConfig{}, fmt.Errorf("unknown fields: %v", fieldNames)
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
	var raw any
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
