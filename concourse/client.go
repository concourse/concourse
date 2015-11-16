package concourse

import (
	"io"

	"github.com/concourse/atc"
)

//go:generate counterfeiter . Client

type Client interface {
	AllBuilds() ([]atc.Build, error)
	Build(buildID string) (atc.Build, bool, error)
	BuildEvents(buildID string) (Events, error)
	BuildResources(buildID int) (atc.BuildInputsOutputs, bool, error)
	AbortBuild(buildID string) error
	BuildInputsForJob(pipelineName string, jobName string) ([]atc.BuildInput, bool, error)
	CreateBuild(plan atc.Plan) (atc.Build, error)
	CreateJobBuild(pipelineName string, jobName string) (atc.Build, error)
	CreateOrUpdatePipelineConfig(pipelineName string, configVersion string, passedConfig atc.Config) (bool, bool, error)
	CreatePipe() (atc.Pipe, error)
	DeletePipeline(pipelineName string) (bool, error)
	PausePipeline(pipelineName string) (bool, error)
	UnpausePipeline(pipelineName string) (bool, error)
	Job(pipelineName, jobName string) (atc.Job, bool, error)
	JobBuild(pipelineName, jobName, buildName string) (atc.Build, bool, error)
	ListContainers(queryList map[string]string) ([]atc.Container, error)
	ListPipelines() ([]atc.Pipeline, error)
	ListVolumes() ([]atc.Volume, error)
	ListWorkers() ([]atc.Worker, error)
	PipelineConfig(pipelineName string) (atc.Config, string, bool, error)
	GetCLIReader(arch, platform string) (io.ReadCloser, error)
	ListAuthMethods() ([]atc.AuthMethod, error)
	AuthToken() (atc.AuthToken, error)
	Pipeline(name string) (atc.Pipeline, bool, error)
}

type client struct {
	connection Connection
}

func NewClient(c Connection) Client {
	return &client{connection: c}
}
