package atc

type Job struct {
	ID int `json:"id"`

	Name                 string `json:"name"`
	PipelineName         string `json:"pipeline_name"`
	TeamName             string `json:"team_name"`
	Paused               bool   `json:"paused,omitempty"`
	FirstLoggedBuildID   int    `json:"first_logged_build_id,omitempty"`
	DisableManualTrigger bool   `json:"disable_manual_trigger,omitempty"`
	NextBuild            *Build `json:"next_build"`
	FinishedBuild        *Build `json:"finished_build"`
	TransitionBuild      *Build `json:"transition_build,omitempty"`
	HasNewInputs         bool   `json:"has_new_inputs,omitempty"`

	Inputs  []JobInput  `json:"inputs,omitempty"`
	Outputs []JobOutput `json:"outputs,omitempty"`

	Groups []string `json:"groups"`
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
