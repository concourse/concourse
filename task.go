package atc

import (
	"fmt"
	"strings"
)

type Build struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	JobName string `json:"job_name"`
	URL     string `json:"url"`
}

type BuildStatus string

const (
	StatusStarted   BuildStatus = "started"
	StatusSucceeded BuildStatus = "succeeded"
	StatusFailed    BuildStatus = "failed"
	StatusErrored   BuildStatus = "errored"
	StatusAborted   BuildStatus = "aborted"
)

type TaskConfig struct {
	// The platform the task must run on (e.g. linux, windows).
	Platform string `json:"platform,omitempty" yaml:"platform,omitempty"`

	// Additional tags to influence which workers the task can run on.
	Tags []string `json:"tags,omitempty"  yaml:"tags,omitempty"`

	// Optional string specifying an image to use for the build. Depending on the
	// platform, this may or may not be required (e.g. Windows/OS X vs. Linux).
	Image string `json:"image,omitempty"   yaml:"image,omitempty"`

	// Parameters to pass to the task via environment variables.
	Params map[string]string `json:"params,omitempty"  yaml:"params,omitempty"`

	// Script to execute.
	Run TaskRunConfig `json:"run,omitempty"     yaml:"run,omitempty"`

	// The set of (logical, name-only) inputs required by the task.
	Inputs []TaskInputConfig `json:"inputs,omitempty"  yaml:"inputs,omitempty"`
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

	if len(a.Tags) > 0 || len(b.Tags) > 0 {
		uniqTags := map[string]struct{}{}

		for _, tag := range a.Tags {
			uniqTags[tag] = struct{}{}
		}

		for _, tag := range b.Tags {
			uniqTags[tag] = struct{}{}
		}

		tags := make([]string, len(uniqTags))
		i := 0
		for tag, _ := range uniqTags {
			tags[i] = tag
			i++
		}

		a.Tags = tags
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

type MetadataField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
