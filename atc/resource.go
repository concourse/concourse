package atc

type Resource struct {
	Name                 string       `json:"name"`
	PipelineID           int          `json:"pipeline_id"`
	PipelineName         string       `json:"pipeline_name"`
	PipelineInstanceVars InstanceVars `json:"pipeline_instance_vars,omitempty"`
	TeamName             string       `json:"team_name"`
	Type                 string       `json:"type"`
	LastChecked          int64        `json:"last_checked,omitempty"`
	Icon                 string       `json:"icon,omitempty"`

	PinnedVersion  Version `json:"pinned_version,omitempty"`
	PinnedInConfig bool    `json:"pinned_in_config,omitempty"`
	PinComment     string  `json:"pin_comment,omitempty"`

	Build *BuildSummary `json:"build,omitempty"`
}
