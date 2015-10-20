package atcclient

import "github.com/concourse/atc"

//go:generate counterfeiter . Handler

type Handler interface {
	// 	DownloadCLI()
	// 	HijackContainer()
	// 	ListJobInputs()
	// 	ReadPipe()
	// 	SaveConfig()
	// 	WritePipe()
	AllBuilds() ([]atc.Build, error)
	Build(buildID string) (atc.Build, bool, error)
	BuildEvents(buildID string) (Events, error)
	AbortBuild(buildID string) error
	BuildInputsForJob(pipelineName string, jobName string) ([]atc.BuildInput, bool, error)
	CreateBuild(plan atc.Plan) (atc.Build, error)
	CreatePipe() (atc.Pipe, error)
	DeletePipeline(pipelineName string) (bool, error)
	Job(pipelineName, jobName string) (atc.Job, bool, error)
	JobBuild(pipelineName, jobName, buildName string) (atc.Build, bool, error)
	ListContainers() ([]atc.Container, error)
	ListPipelines() ([]atc.Pipeline, error)
	PipelineConfig(pipelineName string) (atc.Config, string, bool, error)
}

type AtcHandler struct {
	client Client
}

func NewAtcHandler(c Client) AtcHandler {
	return AtcHandler{client: c}
}
