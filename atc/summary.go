package atc

import "encoding/json"

type JobSummary struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	TeamName string `json:"team_name"`

	PipelineID           int          `json:"pipeline_id"`
	PipelineName         string       `json:"pipeline_name"`
	PipelineInstanceVars InstanceVars `json:"pipeline_instance_vars,omitempty"`

	Paused       bool   `json:"paused,omitempty"`
	PausedBy     string `json:"paused_by,omitempty"`
	PausedAt     int64  `json:"paused_at,omitempty"`
	HasNewInputs bool   `json:"has_new_inputs,omitempty"`

	Groups []string `json:"groups,omitempty"`

	FinishedBuild   *BuildSummary `json:"finished_build,omitempty"`
	NextBuild       *BuildSummary `json:"next_build,omitempty"`
	TransitionBuild *BuildSummary `json:"transition_build,omitempty"`

	Inputs  []JobInputSummary  `json:"inputs,omitempty"`
	Outputs []JobOutputSummary `json:"outputs,omitempty"`
}

type BuildSummary struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	Status BuildStatus `json:"status"`

	StartTime int64 `json:"start_time,omitempty"`
	EndTime   int64 `json:"end_time,omitempty"`

	TeamName string `json:"team_name"`

	PipelineID           int          `json:"pipeline_id"`
	PipelineName         string       `json:"pipeline_name"`
	PipelineInstanceVars InstanceVars `json:"pipeline_instance_vars,omitempty"`

	JobName string `json:"job_name,omitempty"`

	PublicPlan *json.RawMessage `json:"plan,omitempty"`
}

type JobInputSummary struct {
	Name     string   `json:"name"`
	Resource string   `json:"resource"`
	Passed   []string `json:"passed,omitempty"`
	Trigger  bool     `json:"trigger,omitempty"`
}

type JobOutputSummary struct {
	Name     string `json:"name"`
	Resource string `json:"resource"`
}
