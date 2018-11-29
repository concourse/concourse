package atc

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/mitchellh/mapstructure"
)

type TaskConfig struct {
	// The platform the task must run on (e.g. linux, windows).
	Platform string `json:"platform,omitempty" yaml:"platform,omitempty" mapstructure:"platform"`

	// Optional string specifying an image to use for the build. Depending on the
	// platform, this may or may not be required (e.g. Windows/OS X vs. Linux).
	RootfsURI string `json:"rootfs_uri,omitempty" yaml:"rootfs_uri,omitempty" mapstructure:"rootfs_uri"`

	ImageResource *ImageResource `json:"image_resource,omitempty" yaml:"image_resource,omitempty" mapstructure:"image_resource"`

	// Limits to set on the Task Container
	Limits ContainerLimits `json:"container_limits,omitempty" yaml:"container_limits,omitempty" mapstructure:"container_limits"`

	// Parameters to pass to the task via environment variables.
	Params map[string]string `json:"params,omitempty" yaml:"params,omitempty" mapstructure:"params"`

	// Script to execute.
	Run TaskRunConfig `json:"run,omitempty" yaml:"run,omitempty" mapstructure:"run"`

	// The set of (logical, name-only) inputs required by the task.
	Inputs []TaskInputConfig `json:"inputs,omitempty" yaml:"inputs,omitempty" mapstructure:"inputs"`

	// The set of (logical, name-only) outputs provided by the task.
	Outputs []TaskOutputConfig `json:"outputs,omitempty" yaml:"outputs,omitempty" mapstructure:"outputs"`

	// Path to cached directory that will be shared between builds for the same task.
	Caches []CacheConfig `json:"caches,omitempty" yaml:"caches,omitempty" mapstructure:"caches"`
}

type ContainerLimits struct {
	CPU    *uint64 `yaml:"cpu,omitempty" json:"cpu,omitempty"  mapstructure:"cpu"`
	Memory *uint64 `yaml:"memory,omitempty" json:"memory,omitempty"  mapstructure:"memory"`
}

type ImageResource struct {
	Type   string `yaml:"type"   json:"type"   mapstructure:"type"`
	Source Source `yaml:"source" json:"source" mapstructure:"source"`

	Params  *Params  `yaml:"params,omitempty"  json:"params,omitempty"  mapstructure:"params"`
	Version *Version `yaml:"version,omitempty" json:"version,omitempty" mapstructure:"version"`
}

func NewTaskConfig(configBytes []byte) (TaskConfig, error) {
	var untypedInput map[string]interface{}

	if err := yaml.Unmarshal(configBytes, &untypedInput); err != nil {
		return TaskConfig{}, err
	}

	var config TaskConfig
	var metadata mapstructure.Metadata

	msConfig := &mapstructure.DecoderConfig{
		Metadata:         &metadata,
		Result:           &config,
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			SanitizeDecodeHook,
			ContainerLimitsDecodeHook,
		),
	}

	decoder, err := mapstructure.NewDecoder(msConfig)
	if err != nil {
		return TaskConfig{}, err
	}

	if err := decoder.Decode(untypedInput); err != nil {
		return TaskConfig{}, err
	}

	if len(metadata.Unused) > 0 {
		keys := strings.Join(metadata.Unused, ", ")
		return TaskConfig{}, fmt.Errorf("extra keys in the task configuration: %s", keys)
	}

	err = config.Validate()
	if err != nil {
		return TaskConfig{}, err
	}

	return config, nil
}

func (config TaskConfig) Validate() error {
	messages := []string{}

	if config.Platform == "" {
		messages = append(messages, "  missing 'platform'")
	}

	if config.Run.Path == "" {
		messages = append(messages, "  missing path to executable to run")
	}

	messages = append(messages, config.validateInputsAndOutputs()...)

	if len(messages) > 0 {
		return fmt.Errorf("invalid task configuration:\n%s", strings.Join(messages, "\n"))
	}

	return nil
}

func (config TaskConfig) validateInputsAndOutputs() []string {
	messages := []string{}

	messages = append(messages, config.validateInputContainsNames()...)
	messages = append(messages, config.validateOutputContainsNames()...)

	return messages
}

func (config TaskConfig) validateDotPath() []string {
	messages := []string{}

	pathCount := 0
	dotPath := false

	for _, input := range config.Inputs {
		path := strings.TrimPrefix(input.resolvePath(), "./")

		if path == "." {
			dotPath = true
		}

		pathCount++
	}

	for _, output := range config.Outputs {
		path := strings.TrimPrefix(output.resolvePath(), "./")

		if path == "." {
			dotPath = true
		}

		pathCount++
	}

	if pathCount > 1 && dotPath {
		messages = append(messages, "  you may not have more than one input or output when one of them has a path of '.'")
	}

	return messages
}

type pathCounter struct {
	inputCount  map[string]int
	outputCount map[string]int
}

func (counter *pathCounter) foundInBoth(path string) bool {
	_, inputFound := counter.inputCount[path]
	_, outputFound := counter.outputCount[path]

	return inputFound && outputFound
}

func (counter *pathCounter) registerInput(input TaskInputConfig) {
	path := strings.TrimPrefix(input.resolvePath(), "./")

	if val, found := counter.inputCount[path]; !found {
		counter.inputCount[path] = 1
	} else {
		counter.inputCount[path] = val + 1
	}
}

func (counter *pathCounter) registerOutput(output TaskOutputConfig) {
	path := strings.TrimPrefix(output.resolvePath(), "./")

	if val, found := counter.outputCount[path]; !found {
		counter.outputCount[path] = 1
	} else {
		counter.outputCount[path] = val + 1
	}
}

func (config TaskConfig) validateOutputContainsNames() []string {
	messages := []string{}

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
	Path string   `json:"path" yaml:"path"`
	Args []string `json:"args,omitempty" yaml:"args,omitempty"`
	Dir  string   `json:"dir,omitempty" yaml:"dir,omitempty"`

	// The user that the task will run as (defaults to whatever the docker image specifies)
	User string `json:"user,omitempty" yaml:"user,omitempty" mapstructure:"user"`
}

type TaskInputConfig struct {
	Name     string `json:"name" yaml:"name"`
	Path     string `json:"path,omitempty" yaml:"path,omitempty"`
	Optional bool   `json:"optional,omitempty" yaml:"optional,omitempty"`
}

func (input TaskInputConfig) resolvePath() string {
	if input.Path != "" {
		return input.Path
	}
	return input.Name
}

type TaskOutputConfig struct {
	Name string `json:"name" yaml:"name"`
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}

func (output TaskOutputConfig) resolvePath() string {
	if output.Path != "" {
		return output.Path
	}
	return output.Name
}

type MetadataField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type CacheConfig struct {
	Path string `json:"path,omitempty" yaml:"path,omitempty" mapstructure:"path"`
}
