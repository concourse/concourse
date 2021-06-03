package atc

type CausalityBuild struct {
	ID      int         `json:"id"`
	Name    string      `json:"name"`
	JobID   int         `json:"job_id"`
	JobName string      `json:"job_name"`
	Status  BuildStatus `json:"status"`

	ResourceVersions []*CausalityResourceVersion `json:"resource_versions,omitempty"`
}

type CausalityResourceVersion struct {
	ResourceID        int     `json:"resource_id"`
	ResourceVersionID int     `json:"resource_version_id"`
	ResourceName      string  `json:"resource_name"`
	Version           Version `json:"version"`

	Builds []*CausalityBuild `json:"builds,omitempty"`
}
