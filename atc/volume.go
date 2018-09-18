package atc

type VolumeResourceType struct {
	ResourceType     *VolumeResourceType     `json:"resource_type"`
	BaseResourceType *VolumeBaseResourceType `json:"base_resource_type"`
	Version          Version                 `json:"version"`
}

type VolumeBaseResourceType struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Volume struct {
	ID               string                  `json:"id"`
	WorkerName       string                  `json:"worker_name"`
	Type             string                  `json:"type"`
	ContainerHandle  string                  `json:"container_handle"`
	Path             string                  `json:"path"`
	ParentHandle     string                  `json:"parent_handle"`
	ResourceType     *VolumeResourceType     `json:"resource_type"`
	BaseResourceType *VolumeBaseResourceType `json:"base_resource_type"`
	PipelineName     string                  `json:"pipeline_name"`
	JobName          string                  `json:"job_name"`
	StepName         string                  `json:"step_name"`
}
