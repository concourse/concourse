package resource

import "fmt"

type TrackerMetadata struct {
	ExternalURL  string
	PipelineName string
	ResourceName string
}

type EmptyMetadata struct{}

func (m EmptyMetadata) Env() []string {
	return nil
}

func (m TrackerMetadata) Env() []string {
	var env []string

	if m.ExternalURL != "" {
		env = append(env, fmt.Sprintf("ATC_EXTERNAL_URL=%s", m.ExternalURL))
	}

	if m.PipelineName != "" {
		env = append(env, fmt.Sprintf("RESOURCE_PIPELINE_NAME=%s", m.PipelineName))
	}

	if m.ResourceName != "" {
		env = append(env, fmt.Sprintf("RESOURCE_NAME=%s", m.ResourceName))
	}

	return env
}
