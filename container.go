package atc

type Container struct {
	ID         string `json:"id"`
	WorkerName string `json:"worker_name"`

	Type string `json:"type,omitempty"`

	StepName string `json:"step_name,omitempty"`
	Attempt  string `json:"attempt,omitempty"`

	PipelineID     int `json:"pipeline_id,omitempty"`
	JobID          int `json:"job_id,omitempty"`
	BuildID        int `json:"build_id,omitempty"`
	ResourceID     int `json:"resource_id,omitempty"`
	ResourceTypeID int `json:"resource_type_id,omitempty"`

	PipelineName     string `json:"pipeline_name,omitempty"`
	JobName          string `json:"job_name,omitempty"`
	BuildName        string `json:"build_name,omitempty"`
	ResourceName     string `json:"resource_name,omitempty"`
	ResourceTypeName string `json:"resource_type_name,omitempty"`

	User             string `json:"user,omitempty"`
	WorkingDirectory string `json:"working_directory,omitempty"`
}
