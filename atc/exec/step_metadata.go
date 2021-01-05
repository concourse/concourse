package exec

import (
	"encoding/json"
	"fmt"
	"github.com/concourse/concourse/atc"
)

type StepMetadata struct {
	BuildID              int
	BuildName            string
	TeamID               int
	TeamName             string
	JobID                int
	JobName              string
	PipelineID           int
	PipelineName         string
	PipelineInstanceVars map[string]interface{}
	ExternalURL          string
	CreatedBy            *atc.Claims
}

func (metadata StepMetadata) Env() []string {
	env := []string{}

	if metadata.BuildID != 0 {
		env = append(env, fmt.Sprintf("BUILD_ID=%d", metadata.BuildID))
	}

	if metadata.BuildName != "" {
		env = append(env, "BUILD_NAME="+metadata.BuildName)
	}

	if metadata.TeamID != 0 {
		env = append(env, fmt.Sprintf("BUILD_TEAM_ID=%d", metadata.TeamID))
	}

	if metadata.TeamName != "" {
		env = append(env, "BUILD_TEAM_NAME="+metadata.TeamName)
	}

	if metadata.JobID != 0 {
		env = append(env, fmt.Sprintf("BUILD_JOB_ID=%d", metadata.JobID))
	}

	if metadata.JobName != "" {
		env = append(env, "BUILD_JOB_NAME="+metadata.JobName)
	}

	if metadata.PipelineID != 0 {
		env = append(env, fmt.Sprintf("BUILD_PIPELINE_ID=%d", metadata.PipelineID))
	}

	if metadata.PipelineName != "" {
		env = append(env, "BUILD_PIPELINE_NAME="+metadata.PipelineName)
	}

	if metadata.PipelineInstanceVars != nil {
		bytes, _ := json.Marshal(metadata.PipelineInstanceVars)
		env = append(env, "BUILD_PIPELINE_INSTANCE_VARS="+string(bytes))
	}

	if metadata.ExternalURL != "" {
		env = append(env, "ATC_EXTERNAL_URL="+metadata.ExternalURL)
	}

	if metadata.CreatedBy != nil {
		if metadata.CreatedBy.UserID != "" {
			env = append(env, "BUILD_CREATED_BY="+metadata.CreatedBy.UserID)
		} else if metadata.CreatedBy.UserName != "" {
			env = append(env, "BUILD_CREATED_BY="+metadata.CreatedBy.UserName)
		} else if metadata.CreatedBy.PreferredUsername != "" {
			env = append(env, "BUILD_CREATED_BY="+metadata.CreatedBy.PreferredUsername)
		}
	}

	return env
}
