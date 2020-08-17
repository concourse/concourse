package atc

type Resource struct {
	Name         string `json:"name"`
	PipelineName string `json:"pipeline_name"`
	TeamName     string `json:"team_name"`
	Type         string `json:"type"`
	LastChecked  int64  `json:"last_checked,omitempty"`
	Icon         string `json:"icon,omitempty"`

	FailingToCheck  bool   `json:"failing_to_check,omitempty"`
	CheckSetupError string `json:"check_setup_error,omitempty"`
	CheckError      string `json:"check_error,omitempty"`

	PinnedVersion  Version `json:"pinned_version,omitempty"`
	PinnedInConfig bool    `json:"pinned_in_config,omitempty"`
	PinComment     string  `json:"pin_comment,omitempty"`
}
