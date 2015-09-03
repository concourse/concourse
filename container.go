package atc

type PresentedContainer struct {
	ID           string `json:"id"`
	PipelineName string `json:"pipeline"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	BuildID      int    `json:"build_id"`
}

type ListContainersReturn struct {
	Containers []PresentedContainer
	Errors     []string
}
