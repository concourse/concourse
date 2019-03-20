package concourse

import (
	"io"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
)

//go:generate counterfeiter . Team

type Team interface {
	Name() string

	CreateOrUpdate(team atc.Team) (atc.Team, bool, bool, error)
	RenameTeam(teamName, name string) (bool, error)
	DestroyTeam(teamName string) error

	Pipeline(name string) (atc.Pipeline, bool, error)
	PipelineBuilds(pipelineName string, page Page) ([]atc.Build, Pagination, bool, error)
	DeletePipeline(pipelineName string) (bool, error)
	PausePipeline(pipelineName string) (bool, error)
	UnpausePipeline(pipelineName string) (bool, error)
	ExposePipeline(pipelineName string) (bool, error)
	HidePipeline(pipelineName string) (bool, error)
	RenamePipeline(pipelineName, name string) (bool, error)
	ListPipelines() ([]atc.Pipeline, error)
	PipelineConfig(pipelineName string) (atc.Config, string, bool, error)
	CreateOrUpdatePipelineConfig(pipelineName string, configVersion string, passedConfig []byte, checkCredentials bool) (bool, bool, []ConfigWarning, error)

	CreatePipelineBuild(pipelineName string, plan atc.Plan) (atc.Build, error)

	BuildInputsForJob(pipelineName string, jobName string) ([]atc.BuildInput, bool, error)

	Job(pipelineName, jobName string) (atc.Job, bool, error)
	JobBuild(pipelineName, jobName, buildName string) (atc.Build, bool, error)
	JobBuilds(pipelineName string, jobName string, page Page) ([]atc.Build, Pagination, bool, error)
	CreateJobBuild(pipelineName string, jobName string) (atc.Build, error)
	ListJobs(pipelineName string) ([]atc.Job, error)

	PauseJob(pipelineName string, jobName string) (bool, error)
	UnpauseJob(pipelineName string, jobName string) (bool, error)

	ClearTaskCache(pipelineName string, jobName string, stepName string, cachePath string) (int64, error)

	Resource(pipelineName string, resourceName string) (atc.Resource, bool, error)
	ListResources(pipelineName string) ([]atc.Resource, error)
	VersionedResourceTypes(pipelineName string) (atc.VersionedResourceTypes, bool, error)
	ResourceVersions(pipelineName string, resourceName string, page Page) ([]atc.ResourceVersion, Pagination, bool, error)
	CheckResource(pipelineName string, resourceName string, version atc.Version) (bool, error)
	CheckResourceType(pipelineName string, resourceTypeName string, version atc.Version) (bool, error)
	DisableResourceVersion(pipelineName string, resourceName string, resourceVersionID int) (bool, error)
	EnableResourceVersion(pipelineName string, resourceName string, resourceVersionID int) (bool, error)

	BuildsWithVersionAsInput(pipelineName string, resourceName string, resourceVersionID int) ([]atc.Build, bool, error)
	BuildsWithVersionAsOutput(pipelineName string, resourceName string, resourceVersionID int) ([]atc.Build, bool, error)

	ListContainers(queryList map[string]string) ([]atc.Container, error)
	GetContainer(id string) (atc.Container, error)
	ListVolumes() ([]atc.Volume, error)
	CreateBuild(plan atc.Plan) (atc.Build, error)
	Builds(page Page) ([]atc.Build, Pagination, error)
	OrderingPipelines(pipelineNames []string) error

	CreateArtifact(io.Reader) (atc.WorkerArtifact, error)
	GetArtifact(int) (io.ReadCloser, error)
}

type team struct {
	name       string
	connection internal.Connection
}

func (team *team) Name() string {
	return team.name
}

func (client *client) Team(name string) Team {
	return &team{
		name:       name,
		connection: client.connection,
	}
}
