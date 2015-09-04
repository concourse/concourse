package atc

type PresentedContainer struct {
	ID           string `json:"id"`
	PipelineName string `json:"pipeline_name"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	BuildID      int    `json:"build_id"`
}

type ListContainersReturn struct {
	Containers []PresentedContainer `json:"containers"`
	Errors     []string             `json:"errors"`
}
