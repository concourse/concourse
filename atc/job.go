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

	Inputs  []JobInput  `json:"inputs"`
	Outputs []JobOutput `json:"outputs"`

	Groups []string `json:"groups"`
}

type JobInput struct {
	Name     string         `json:"name"`
	Resource string         `json:"resource"`
	Passed   []string       `json:"passed,omitempty"`
	Trigger  bool           `json:"trigger"`
	Version  *VersionConfig `json:"version,omitempty"`
	Params   Params         `json:"params,omitempty"`
	Tags     Tags           `json:"tags,omitempty"`
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
