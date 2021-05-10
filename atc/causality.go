package atc

type CausalityBuild struct {
	ID      int                         `json:"id"`
	Name    string                      `json:"name"`
	JobID   int                         `json:"job_id"`
	JobName string                      `json:"job_name"`
	Outputs []*CausalityResourceVersion `json:"outputs,omitempty"`
}

type CausalityResourceVersion struct {
	ResourceID   int               `json:"resource_id"`
	ResourceName string            `json:"resource_name"`
	Version      Version           `json:"version"`
	InputTo      []*CausalityBuild `json:"input_to,omitempty"`
}
