package atcclient

import "github.com/concourse/atc"

//go:generate counterfeiter . Handler
type Handler interface {
	// 	BuildEvents()
	// 	DownloadCLI()
	// 	HijackContainer()
	// 	ListJobInputs()
	// 	ReadPipe()
	// 	SaveConfig()
	// 	WritePipe()
	AbortBuild(buildID string) error
	AllBuilds() ([]atc.Build, error)
	Build(buildID string) (atc.Build, error)
	BuildInputsForJob(pipelineName string, jobName string) ([]atc.BuildInput, error)
	CreateBuild(plan atc.Plan) (atc.Build, error)
	CreatePipe() (atc.Pipe, error)
	DeletePipeline(pipelineName string) error
	Job(pipelineName, jobName string) (atc.Job, error)
	JobBuild(pipelineName, jobName, buildName string) (atc.Build, error)
	ListContainers() ([]atc.Container, error)
	ListPipelines() ([]atc.Pipeline, error)
	PipelineConfig(pipelineName string) (atc.Config, string, error)
}

type AtcHandler struct {
	client Client
}

func NewAtcHandler(c Client) AtcHandler {
	return AtcHandler{client: c}
}
