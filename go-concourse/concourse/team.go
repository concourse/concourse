package concourse

import (
	"io"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse/internal"
)

//counterfeiter:generate . Team
type Team interface {
	Name() string
	ID() int
	Auth() atc.TeamAuth
	ATCTeam() atc.Team

	CreateOrUpdate(team atc.Team) (atc.Team, bool, bool, []ConfigWarning, error)
	RenameTeam(teamName, name string) (bool, []ConfigWarning, error)
	DestroyTeam(teamName string) error

	Pipeline(pipelineRef atc.PipelineRef) (atc.Pipeline, bool, error)
	PipelineBuilds(pipelineRef atc.PipelineRef, page Page) ([]atc.Build, Pagination, bool, error)
	DeletePipeline(pipelineRef atc.PipelineRef) (bool, error)
	PausePipeline(pipelineRef atc.PipelineRef) (bool, error)
	ArchivePipeline(pipelineRef atc.PipelineRef) (bool, error)
	UnpausePipeline(pipelineRef atc.PipelineRef) (bool, error)
	ExposePipeline(pipelineRef atc.PipelineRef) (bool, error)
	HidePipeline(pipelineRef atc.PipelineRef) (bool, error)
	RenamePipeline(oldName, newName string) (bool, []ConfigWarning, error)
	ListPipelines() ([]atc.Pipeline, error)
	PipelineConfig(pipelineRef atc.PipelineRef) (atc.Config, string, bool, error)
	CreateOrUpdatePipelineConfig(pipelineRef atc.PipelineRef, configVersion string, passedConfig []byte, checkCredentials bool) (bool, bool, []ConfigWarning, error)

	CreatePipelineBuild(pipelineRef atc.PipelineRef, plan atc.Plan) (atc.Build, error)

	BuildInputsForJob(pipelineRef atc.PipelineRef, jobName string) ([]atc.BuildInput, bool, error)

	Job(pipelineRef atc.PipelineRef, jobName string) (atc.Job, bool, error)
	JobBuild(pipelineRef atc.PipelineRef, jobName, buildName string) (atc.Build, bool, error)
	JobBuilds(pipelineRef atc.PipelineRef, jobName string, page Page) ([]atc.Build, Pagination, bool, error)
	CreateJobBuild(pipelineRef atc.PipelineRef, jobName string) (atc.Build, error)
	RerunJobBuild(pipelineRef atc.PipelineRef, jobName string, buildName string) (atc.Build, error)
	SetJobBuildComment(pipelineRef atc.PipelineRef, jobName string, buildName string, comment string) (bool, error)
	ListJobs(pipelineRef atc.PipelineRef) ([]atc.Job, error)
	ScheduleJob(pipelineRef atc.PipelineRef, jobName string) (bool, error)

	PauseJob(pipelineRef atc.PipelineRef, jobName string) (bool, error)
	UnpauseJob(pipelineRef atc.PipelineRef, jobName string) (bool, error)

	ClearTaskCache(pipelineRef atc.PipelineRef, jobName string, stepName string, cachePath string) (int64, error)

	Resource(pipelineRef atc.PipelineRef, resourceName string) (atc.Resource, bool, error)
	ListResources(pipelineRef atc.PipelineRef) ([]atc.Resource, error)
	ResourceTypes(pipelineRef atc.PipelineRef) (atc.ResourceTypes, bool, error)
	ResourceVersions(pipelineRef atc.PipelineRef, resourceName string, page Page, filter atc.Version) ([]atc.ResourceVersion, Pagination, bool, error)
	CheckResource(pipelineRef atc.PipelineRef, resourceName string, version atc.Version, shallow bool) (atc.Build, bool, error)
	CheckResourceType(pipelineRef atc.PipelineRef, resourceTypeName string, version atc.Version, shallow bool) (atc.Build, bool, error)
	CheckPrototype(pipelineRef atc.PipelineRef, prototypeName string, version atc.Version, shallow bool) (atc.Build, bool, error)
	DisableResourceVersion(pipelineRef atc.PipelineRef, resourceName string, resourceVersionID int) (bool, error)
	EnableResourceVersion(pipelineRef atc.PipelineRef, resourceName string, resourceVersionID int) (bool, error)
	ClearResourceCache(pipelineRef atc.PipelineRef, ResourceName string, version atc.Version) (int64, error)

	PinResourceVersion(pipelineRef atc.PipelineRef, resourceName string, resourceVersionID int) (bool, error)
	UnpinResource(pipelineRef atc.PipelineRef, resourceName string) (bool, error)
	SetPinComment(pipelineRef atc.PipelineRef, resourceName string, comment string) (bool, error)

	BuildsWithVersionAsInput(pipelineRef atc.PipelineRef, resourceName string, resourceVersionID int) ([]atc.Build, bool, error)
	BuildsWithVersionAsOutput(pipelineRef atc.PipelineRef, resourceName string, resourceVersionID int) ([]atc.Build, bool, error)

	ListContainers(queryList map[string]string) ([]atc.Container, error)
	GetContainer(id string) (atc.Container, error)
	ListVolumes() ([]atc.Volume, error)
	CreateBuild(plan atc.Plan) (atc.Build, error)
	Builds(page Page) ([]atc.Build, Pagination, error)
	OrderingPipelines(pipelineNames []string) error
	OrderingPipelinesWithinGroup(groupName string, instanceVars []atc.InstanceVars) error

	CreateArtifact(io.Reader, string, []string) (atc.WorkerArtifact, error)
	GetArtifact(int) (io.ReadCloser, error)
}

type team struct {
	atcTeam    atc.Team
	connection internal.Connection //Deprecated
	httpAgent  internal.HTTPAgent
}

func (team *team) Name() string {
	return team.atcTeam.Name
}

func (team *team) ID() int {
	return team.atcTeam.ID
}

func (team *team) Auth() atc.TeamAuth {
	return team.atcTeam.Auth
}

func (team *team) ATCTeam() atc.Team {
	return team.atcTeam
}

func (client *client) Team(name string) Team {
	return &team{
		atcTeam:    atc.Team{Name: name},
		connection: client.connection,
		httpAgent:  client.httpAgent,
	}
}
