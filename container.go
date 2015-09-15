package atc

type Container struct {
	ID           string `json:"id"`
	PipelineName string `json:"pipeline_name"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	BuildID      int    `json:"build_id"`
	WorkerName   string `json:"worker_name"`
}
