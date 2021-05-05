package atc

type CausalityBuild struct {
	ID      int    `json:"id"`
	JobID   int    `json:"job_id"`
	Name    string `json:"name"`
	JobName string `json:"job_name,omitempty"`
	// URL     string                      `json:"url"`
	Outputs []*CausalityResourceVersion `json:"outputs"`
}

// type CausalityJob struct {
// 	ID     int              `json:"id"`
// 	Name   string           `json:"name"`
// 	Builds []CausalityBuild `json:"builds"`
// }

type CausalityResourceVersion struct {
	ResourceID int `json:"resource_id"`
	// ResourceConfigVersionID int    `json:"resource_config_version_id"`
	ResourceName string `json:"resource_name"`
	Version      string `json:"version"`
	// URL                     string            `json:"url"`
	InputTo []*CausalityBuild `json:"input_to"`
}
