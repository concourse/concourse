package atc

type CausalityJob struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	BuildIDs []int `json:"build_ids,omitempty"`
}

type CausalityBuild struct {
	ID     int         `json:"id"`
	Name   string      `json:"name"`
	JobId  int         `json:"job_id"`
	Status BuildStatus `json:"status"`

	ResourceVersionIDs []int `json:"resource_version_ids,omitempty"`
}

type CausalityResource struct {
	ID   int    `json:"id"`
	Name string `json:"name"`

	VersionIDs []int `json:"version_ids,omitempty"`
}

type CausalityResourceVersion struct {
	ID         int     `json:"id"`
	ResourceID int     `json:"resource_id"`
	Version    Version `json:"version"`

	BuildIDs []int `json:"build_ids,omitempty"`
}

type Causality struct {
	Jobs             []CausalityJob             `json:"jobs"`
	Builds           []CausalityBuild           `json:"builds"`
	Resources        []CausalityResource        `json:"resources"`
	ResourceVersions []CausalityResourceVersion `json:"resource_versions"`
}
