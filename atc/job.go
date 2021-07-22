package atc

type Job struct {
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

	FirstLoggedBuildID   int  `json:"first_logged_build_id,omitempty"`
	DisableManualTrigger bool `json:"disable_manual_trigger,omitempty"`

	NextBuild       *Build `json:"next_build"`
	FinishedBuild   *Build `json:"finished_build"`
	TransitionBuild *Build `json:"transition_build,omitempty"`

	Inputs  []JobInput  `json:"inputs,omitempty"`
	Outputs []JobOutput `json:"outputs,omitempty"`
}

type JobInput struct {
	Name     string         `json:"name"`
	Resource string         `json:"resource"`
	Trigger  bool           `json:"trigger"`
	Passed   []string       `json:"passed,omitempty"`
	Version  *VersionConfig `json:"version,omitempty"`
}

type JobInputParams struct {
	JobInput
	Params Params `json:"params,omitempty"`
	Tags   Tags   `json:"tags,omitempty"`
}

type JobOutput struct {
	Name     string `json:"name"`
	Resource string `json:"resource"`
}

type BuildInput struct {
	Name     string   `json:"name"`
	Resource string   `json:"resource"`
	Type     string   `json:"type"`
	Source   Source   `json:"source"`
	Params   Params   `json:"params,omitempty"`
	Version  Version  `json:"version"`
	Tags     []string `json:"tags,omitempty"`
}
