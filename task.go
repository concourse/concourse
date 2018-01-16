package atc

import (
	"fmt"
	"path/filepath"
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
		DecodeHook:       SanitizeDecodeHook,
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

func (config TaskConfig) Merge(other TaskConfig) TaskConfig {
	if other.Platform != "" {
		config.Platform = other.Platform
	}

	if other.RootfsURI != "" {
		config.RootfsURI = other.RootfsURI
	}

	if len(config.Params) > 0 {
		newParams := map[string]string{}

		for k, v := range config.Params {
			newParams[k] = v
		}

		for k, v := range other.Params {
			newParams[k] = v
		}

		config.Params = newParams
	} else {
		config.Params = other.Params
	}

	if len(other.Inputs) != 0 {
		config.Inputs = other.Inputs
	}

	if other.Run.Path != "" {
		config.Run = other.Run
	}

	return config
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
	messages = append(messages, config.validateDotPath()...)
	messages = append(messages, config.validateOverlappingPaths()...)

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

const duplicateErrorMessage = "  cannot have more than one %s using the same path '%s'"

func pathContains(child string, parent string) bool {
	if child == parent {
		return false
	}

	childParts := strings.Split(child, string(filepath.Separator))
	parentParts := strings.Split(parent, string(filepath.Separator))

	if len(childParts) < len(parentParts) {
		return false
	}

	for i, part := range parentParts {
		if childParts[i] == part {
			continue
		} else {
			return false
		}
	}

	return true
}

func (counter pathCounter) getErrorMessages() []string {
	messages := []string{}

	for path, numOccurrences := range counter.inputCount {
		if numOccurrences > 1 {
			messages = append(messages, fmt.Sprintf(duplicateErrorMessage, "input", path))
		}

		if counter.foundInBoth(path) {
			messages = append(messages, fmt.Sprintf("  cannot have an input and output using the same path '%s'", path))
		}

		for candidateParentPath := range counter.inputCount {
			if pathContains(path, candidateParentPath) {
				messages = append(messages, fmt.Sprintf("  cannot nest inputs: '%s' is nested under input directory '%s'", path, candidateParentPath))
			}
		}

		for candidateParentPath := range counter.outputCount {
			if pathContains(path, candidateParentPath) {
				messages = append(messages, fmt.Sprintf("  cannot nest inputs within outputs: '%s' is nested under output directory '%s'", path, candidateParentPath))
			}
		}
	}

	for path, numOccurrences := range counter.outputCount {
		if numOccurrences > 1 {
			messages = append(messages, fmt.Sprintf(duplicateErrorMessage, "output", path))
		}

		for candidateParentPath := range counter.outputCount {
			if pathContains(path, candidateParentPath) {
				messages = append(messages, fmt.Sprintf("  cannot nest outputs: '%s' is nested under output directory '%s'", path, candidateParentPath))
			}
		}

		for candidateParentPath := range counter.inputCount {
			if pathContains(path, candidateParentPath) {
				messages = append(messages, fmt.Sprintf("  cannot nest outputs within inputs: '%s' is nested under input directory '%s'", path, candidateParentPath))
			}
		}

	}

	return messages
}

func (config TaskConfig) countInputOutputPaths() pathCounter {
	counter := &pathCounter{
		inputCount:  make(map[string]int),
		outputCount: make(map[string]int),
	}

	for _, input := range config.Inputs {
		counter.registerInput(input)
	}

	for _, output := range config.Outputs {
		counter.registerOutput(output)
	}

	return *counter
}

func (config TaskConfig) validateOverlappingPaths() []string {
	counter := config.countInputOutputPaths()
	return counter.getErrorMessages()
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
	Path     string `json:"path,omitempty" yaml:"path"`
	Optional bool   `json:"optional,omitempty" yaml:"optional"`
}

func (input TaskInputConfig) resolvePath() string {
	if input.Path != "" {
		return input.Path
	}
	return input.Name
}

type TaskOutputConfig struct {
	Name string `json:"name" yaml:"name"`
	Path string `json:"path,omitempty" yaml:"path"`
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
