package atcclient

import "github.com/concourse/atc"

//go:generate counterfeiter . Handler
type Handler interface {
	// 	AbortBuild()
	// 	BuildEvents()
	// 	CreateBuild()
	// 	CreatePipe()
	// 	DeletePipeline()
	// 	DownloadCLI()
	// 	HijackContainer()
	// 	ListContainer()
	// 	ListJobInputs()
	// 	ReadPipe()
	// 	SaveConfig()
	// 	WritePipe()
	AllBuilds() ([]atc.Build, error)
	Build(buildID string) (atc.Build, error)
	Job(pipelineName, jobName string) (atc.Job, error)
	JobBuild(pipelineName, jobName, buildName string) (atc.Build, error)
	PipelineConfig(pipelineName string) (atc.Config, error)
}

type AtcHandler struct {
	client Client
}

func NewAtcHandler(c Client) AtcHandler {
	return AtcHandler{client: c}
}
