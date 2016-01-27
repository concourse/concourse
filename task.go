package atc

import (
	"fmt"
	"strings"
)

type TaskConfig struct {
	// The platform the task must run on (e.g. linux, windows).
	Platform string `json:"platform,omitempty" yaml:"platform,omitempty"`

	// Optional string specifying an image to use for the build. Depending on the
	// platform, this may or may not be required (e.g. Windows/OS X vs. Linux).
	Image string `json:"image,omitempty"   yaml:"image,omitempty"`

	ImageResource *TaskImageConfig `json:"image_resource,omitempty" yaml:"image_resource,omitempty"`

	// Parameters to pass to the task via environment variables.
	Params map[string]string `json:"params,omitempty"  yaml:"params,omitempty"`

	// Script to execute.
	Run TaskRunConfig `json:"run,omitempty"     yaml:"run,omitempty"`

	// The set of (logical, name-only) inputs required by the task.
	Inputs []TaskInputConfig `json:"inputs,omitempty"  yaml:"inputs,omitempty"`

	// The set of (logical, name-only) outputs provided by the task.
	Outputs []TaskOutputConfig `json:"outputs,omitempty"  yaml:"outputs,omitempty"`
}

type TaskImageConfig struct {
	Type   string `yaml:"type" json:"type" mapstructure:"type"`
	Source Source `yaml:"source" json:"source" mapstructure:"source"`
}

func (config TaskConfig) Merge(other TaskConfig) TaskConfig {
	if other.Platform != "" {
		config.Platform = other.Platform
	}

	if other.Image != "" {
		config.Image = other.Image
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

	messages = append(messages, config.validateInputContainsNames()...)
	messages = append(messages, config.validateOutputContainsNames()...)
	messages = append(messages, config.validateInputsAndOutputs()...)

	if len(messages) > 0 {
		return fmt.Errorf("invalid task configuration:\n%s", strings.Join(messages, "\n"))
	}

	return nil
}

func (config TaskConfig) validateInputsAndOutputs() []string {
	messages := []string{}
	previousPaths := make(map[string]int)
	dotPath := false

	for _, input := range config.Inputs {
		path := strings.TrimPrefix(input.resolvePath(), "./")

		if path == "." {
			dotPath = true
		}

		if val, found := previousPaths[path]; !found {
			previousPaths[path] = 1
		} else {
			previousPaths[path] = val + 1
		}
	}

	for _, output := range config.Outputs {
		path := strings.TrimPrefix(output.resolvePath(), "./")

		if path == "." {
			dotPath = true
		}

		if val, found := previousPaths[path]; !found {
			previousPaths[path] = 1
		} else {
			previousPaths[path] = val + 1
		}
	}

	if len(previousPaths) > 1 && dotPath {
		messages = append(messages, "  you may not have more than one input or output when one of them has a path of '.'")
	}

	for path, val := range previousPaths {
		if val > 1 {
			messages = append(messages, fmt.Sprintf("  inputs and/or outputs have overlapping path: '%s'", path))
		}

		for _, input := range config.Inputs {
			inputPath := strings.TrimPrefix(input.resolvePath(), "./")

			if strings.HasPrefix(inputPath, path) && inputPath != path {
				messages = append(messages, fmt.Sprintf("  inputs and/or outputs have overlapping path: '%s'", path))
			}
		}

		for _, output := range config.Outputs {
			outputPath := strings.TrimPrefix(output.resolvePath(), "./")

			if strings.HasPrefix(outputPath, path) && outputPath != path {
				messages = append(messages, fmt.Sprintf("  inputs and/or outputs have overlapping path: '%s'", path))
			}
		}
	}

	return messages
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
	Args []string `json:"args,omitempty" yaml:"args"`
}

type TaskInputConfig struct {
	Name string `json:"name" yaml:"name"`
	Path string `json:"path,omitempty" yaml:"path"`
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
