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

	ImageResource *TaskImageConfig `json:"image_resource,omitempty"	yaml:"image_resource,omitempty"`

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

func (a TaskConfig) Merge(b TaskConfig) TaskConfig {
	if b.Platform != "" {
		a.Platform = b.Platform
	}

	if b.Image != "" {
		a.Image = b.Image
	}

	if len(a.Params) > 0 {
		newParams := map[string]string{}

		for k, v := range a.Params {
			newParams[k] = v
		}

		for k, v := range b.Params {
			newParams[k] = v
		}

		a.Params = newParams
	} else {
		a.Params = b.Params
	}

	if len(b.Inputs) != 0 {
		a.Inputs = b.Inputs
	}

	if b.Run.Path != "" {
		a.Run = b.Run
	}

	return a
}

func (config TaskConfig) Validate() error {
	messages := []string{"invalid task configuration:"}

	var invalid bool
	if config.Platform == "" {
		messages = append(messages, "  missing 'platform'")
		invalid = true
	}

	if config.Run.Path == "" {
		messages = append(messages, "  missing path to executable to run")
		invalid = true
	}

	if invalid {
		return fmt.Errorf(strings.Join(messages, "\n"))
	}

	return nil
}

type TaskRunConfig struct {
	Path string   `json:"path" yaml:"path"`
	Args []string `json:"args,omitempty" yaml:"args"`
}

type TaskInputConfig struct {
	Name string `json:"name" yaml:"name"`
	Path string `json:"path,omitempty" yaml:"path"`
}

type TaskOutputConfig struct {
	Name string `json:"name" yaml:"name"`
	Path string `json:"path,omitempty" yaml:"path"`
}

type MetadataField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
