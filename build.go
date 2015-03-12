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

type BuildPlan struct {
	Privileged bool `json:"privileged"`

	Config     *TaskConfig `json:"config"`
	ConfigPath string      `json:"config_path"`

	Inputs  []InputPlan  `json:"inputs"`
	Outputs []OutputPlan `json:"outputs"`
}

type TaskConfig struct {
	Platform string             `json:"platform,omitempty" yaml:"platform"`
	Tags     []string           `json:"tags,omitempty"  yaml:"tags"`
	Image    string             `json:"image,omitempty"   yaml:"image"`
	Params   map[string]string  `json:"params,omitempty"  yaml:"params"`
	Run      BuildRunConfig     `json:"run,omitempty"     yaml:"run"`
	Inputs   []BuildInputConfig `json:"inputs,omitempty"  yaml:"inputs"`
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

type BuildRunConfig struct {
	Path string   `json:"path" yaml:"path"`
	Args []string `json:"args,omitempty" yaml:"args"`
}

type BuildInputConfig struct {
	Name string `json:"name" yaml:"name"`
	Path string `json:"path,omitempty" yaml:"path"`
}

type InputPlan struct {
	// logical name of the input with respect to the task's config
	Name string `json:"name"`

	// name of resource providing the input
	Resource string `json:"resource"`

	// type of resource
	Type string `json:"type"`

	// e.g. sha
	Version Version `json:"version,omitempty"`

	// e.g. git url, branch, private_key
	Source Source `json:"source,omitempty"`

	// arbitrary config for input
	Params Params `json:"params,omitempty"`
}

type OutputPlan struct {
	Name string `json:"name"`

	Type string `json:"type"`

	// e.g. [success, failure]
	On Conditions `json:"on,omitempty"`

	// e.g. git url, branch, private_key
	Source Source `json:"source,omitempty"`

	// arbitrary config for output
	Params Params `json:"params,omitempty"`
}

type MetadataField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
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
