package engine

import "fmt"

type StepMetadata struct {
	BuildID int

	PipelineName string
	JobName      string
	BuildName    string
	ExternalURL  string
	TeamName     string
}

func (metadata StepMetadata) Env() []string {
	env := []string{fmt.Sprintf("BUILD_ID=%d", metadata.BuildID)}

	if metadata.PipelineName != "" {
		env = append(env, "BUILD_PIPELINE_NAME="+metadata.PipelineName)
	}

	if metadata.JobName != "" {
		env = append(env, "BUILD_JOB_NAME="+metadata.JobName)
	}

	if metadata.BuildName != "" {
		env = append(env, "BUILD_NAME="+metadata.BuildName)
	}

	if metadata.ExternalURL != "" {
		env = append(env, "ATC_EXTERNAL_URL="+metadata.ExternalURL)
	}

	if metadata.TeamName != "" {
		env = append(env, "BUILD_TEAM_NAME="+metadata.TeamName)
	}

	return env
}
