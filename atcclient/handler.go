package atcclient

import (
	"bytes"
	"io"

	"github.com/concourse/atc"
)

//go:generate counterfeiter . Handler

type Handler interface {
	// 	HijackContainer()
	// 	ListJobInputs()
	// 	ReadPipe()
	// 	WritePipe()
	AllBuilds() ([]atc.Build, error)
	Build(buildID string) (atc.Build, bool, error)
	BuildEvents(buildID string) (Events, error)
	AbortBuild(buildID string) error
	BuildInputsForJob(pipelineName string, jobName string) ([]atc.BuildInput, bool, error)
	CreateBuild(plan atc.Plan) (atc.Build, error)
	CreateOrUpdatePipelineConfig(pipelineName string, configVersion string, buffer *bytes.Buffer, contentType string) (bool, bool, error)
	CreatePipe() (atc.Pipe, error)
	DeletePipeline(pipelineName string) (bool, error)
	Job(pipelineName, jobName string) (atc.Job, bool, error)
	JobBuild(pipelineName, jobName, buildName string) (atc.Build, bool, error)
	ListContainers(queryList map[string]string) ([]atc.Container, error)
	ListPipelines() ([]atc.Pipeline, error)
	ListVolumes() ([]atc.Volume, error)
	ListWorkers() ([]atc.Worker, error)
	PipelineConfig(pipelineName string) (atc.Config, string, bool, error)
	GetCLIReader(arch, platform string) (io.ReadCloser, error)
}

type AtcHandler struct {
	client Client
}

func NewAtcHandler(c Client) AtcHandler {
	return AtcHandler{client: c}
}
