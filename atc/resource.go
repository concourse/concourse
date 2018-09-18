package atc

type Resource struct {
	Name         string `json:"name"`
	PipelineName string `json:"pipeline_name"`
	TeamName     string `json:"team_name"`
	Type         string `json:"type"`
	LastChecked  int64  `json:"last_checked,omitempty"`

	Paused bool `json:"paused,omitempty"`

	FailingToCheck bool   `json:"failing_to_check,omitempty"`
	CheckError     string `json:"check_error,omitempty"`
}
