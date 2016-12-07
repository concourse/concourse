package atc

type Container struct {
	ID                   string   `json:"id"`
	WorkerName           string   `json:"worker_name"`
	PipelineName         string   `json:"pipeline_name"`
	JobName              string   `json:"job_name,omitempty"`
	BuildName            string   `json:"build_name,omitempty"`
	BuildID              int      `json:"build_id,omitempty"`
	StepType             string   `json:"step_type,omitempty"`
	StepName             string   `json:"step_name,omitempty"`
	ResourceName         string   `json:"resource_name,omitempty"`
	WorkingDirectory     string   `json:"working_directory,omitempty"`
	EnvironmentVariables []string `json:"env_variables,omitempty"`
	Attempts             []int    `json:"attempt,omitempty"`
	User                 string   `json:"user,omitempty"`
}
