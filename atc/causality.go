package atc

type CausalityBuild struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	JobID   int    `json:"job_id"`
	JobName string `json:"job_name"`

	Inputs  []*CausalityResourceVersion `json:"inputs,omitempty"`
	Outputs []*CausalityResourceVersion `json:"outputs,omitempty"`
}

type CausalityResourceVersion struct {
	ResourceID        int     `json:"resource_id"`
	ResourceVersionID int     `json:"resource_version_id"`
	ResourceName      string  `json:"resource_name"`
	Version           Version `json:"version"`

	InputTo  []*CausalityBuild `json:"input_to,omitempty"`
	OutputOf []*CausalityBuild `json:"output_of,omitempty"`
}
