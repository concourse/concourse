package atc

import "github.com/tedsuo/rata"

const (
	SaveConfig = "SaveConfig"
	GetConfig  = "GetConfig"

	GetBuild            = "GetBuild"
	GetBuildPlan        = "GetBuildPlan"
	CreateBuild         = "CreateBuild"
	ListBuilds          = "ListBuilds"
	BuildEvents         = "BuildEvents"
	BuildResources      = "BuildResources"
	AbortBuild          = "AbortBuild"
	GetBuildPreparation = "GetBuildPreparation"
	SetBuildComment     = "SetBuildComment"

	GetJob         = "GetJob"
	CreateJobBuild = "CreateJobBuild"
	RerunJobBuild  = "RerunJobBuild"
	ListAllJobs    = "ListAllJobs"
	ListJobs       = "ListJobs"
	ListJobBuilds  = "ListJobBuilds"
	ListJobInputs  = "ListJobInputs"
	GetJobBuild    = "GetJobBuild"
	PauseJob       = "PauseJob"
	UnpauseJob     = "UnpauseJob"
	ScheduleJob    = "ScheduleJob"
	GetVersionsDB  = "GetVersionsDB"
	JobBadge       = "JobBadge"
	MainJobBadge   = "MainJobBadge"

	ClearTaskCache = "ClearTaskCache"

	ListAllResources     = "ListAllResources"
	ListResources        = "ListResources"
	ListResourceTypes    = "ListResourceTypes"
	GetResource          = "GetResource"
	CheckResource        = "CheckResource"
	CheckResourceWebHook = "CheckResourceWebHook"
	CheckResourceType    = "CheckResourceType"
	CheckPrototype       = "CheckPrototype"

	ListResourceVersions           = "ListResourceVersions"
	GetResourceVersion             = "GetResourceVersion"
	EnableResourceVersion          = "EnableResourceVersion"
	DisableResourceVersion         = "DisableResourceVersion"
	PinResourceVersion             = "PinResourceVersion"
	UnpinResource                  = "UnpinResource"
	SetPinCommentOnResource        = "SetPinCommentOnResource"
	ListBuildsWithVersionAsInput   = "ListBuildsWithVersionAsInput"
	ListBuildsWithVersionAsOutput  = "ListBuildsWithVersionAsOutput"
	ClearResourceCache             = "ClearResourceCache"
	GetDownstreamResourceCausality = "GetDownstreamResourceCausality"
	GetUpstreamResourceCausality   = "GetUpstreamResourceCausality"

	GetCC = "GetCC"

	ListAllPipelines          = "ListAllPipelines"
	ListPipelines             = "ListPipelines"
	GetPipeline               = "GetPipeline"
	DeletePipeline            = "DeletePipeline"
	OrderPipelines            = "OrderPipelines"
	OrderPipelinesWithinGroup = "OrderPipelinesWithinGroup"
	PausePipeline             = "PausePipeline"
	ArchivePipeline           = "ArchivePipeline"
	UnpausePipeline           = "UnpausePipeline"
	ExposePipeline            = "ExposePipeline"
	HidePipeline              = "HidePipeline"
	RenamePipeline            = "RenamePipeline"
	ListPipelineBuilds        = "ListPipelineBuilds"
	CreatePipelineBuild       = "CreatePipelineBuild"
	PipelineBadge             = "PipelineBadge"

	RegisterWorker  = "RegisterWorker"
	LandWorker      = "LandWorker"
	RetireWorker    = "RetireWorker"
	PruneWorker     = "PruneWorker"
	HeartbeatWorker = "HeartbeatWorker"
	ListWorkers     = "ListWorkers"
	DeleteWorker    = "DeleteWorker"

	SetLogLevel = "SetLogLevel"
	GetLogLevel = "GetLogLevel"

	DownloadCLI  = "DownloadCLI"
	GetInfo      = "GetInfo"
	GetInfoCreds = "GetInfoCreds"

	ListContainers           = "ListContainers"
	GetContainer             = "GetContainer"
	HijackContainer          = "HijackContainer"
	ListDestroyingContainers = "ListDestroyingContainers"
	ReportWorkerContainers   = "ReportWorkerContainers"

	ListVolumes           = "ListVolumes"
	ListDestroyingVolumes = "ListDestroyingVolumes"
	ReportWorkerVolumes   = "ReportWorkerVolumes"

	ListTeams      = "ListTeams"
	GetTeam        = "GetTeam"
	SetTeam        = "SetTeam"
	RenameTeam     = "RenameTeam"
	DestroyTeam    = "DestroyTeam"
	ListTeamBuilds = "ListTeamBuilds"

	CreateArtifact     = "CreateArtifact"
	GetArtifact        = "GetArtifact"
	ListBuildArtifacts = "ListBuildArtifacts"

	GetUser              = "GetUser"
	ListActiveUsersSince = "ListActiveUsersSince"

	SetWall   = "SetWall"
	GetWall   = "GetWall"
	ClearWall = "ClearWall"
)

const (
	ClearTaskCacheQueryPath = "cache_path"
	SaveConfigCheckCreds    = "check_creds"
)

var Routes = rata.Routes([]rata.Route{
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/config", Method: "PUT", Name: SaveConfig},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/config", Method: "GET", Name: GetConfig},

	{Path: "/api/v1/teams/:team_name/builds", Method: "POST", Name: CreateBuild},

	{Path: "/api/v1/builds", Method: "GET", Name: ListBuilds},
	{Path: "/api/v1/builds/:build_id", Method: "GET", Name: GetBuild},
	{Path: "/api/v1/builds/:build_id/plan", Method: "GET", Name: GetBuildPlan},
	{Path: "/api/v1/builds/:build_id/events", Method: "GET", Name: BuildEvents},
	{Path: "/api/v1/builds/:build_id/resources", Method: "GET", Name: BuildResources},
	{Path: "/api/v1/builds/:build_id/abort", Method: "PUT", Name: AbortBuild},
	{Path: "/api/v1/builds/:build_id/preparation", Method: "GET", Name: GetBuildPreparation},
	{Path: "/api/v1/builds/:build_id/artifacts", Method: "GET", Name: ListBuildArtifacts},
	{Path: "/api/v1/builds/:build_id/comment", Method: "PUT", Name: SetBuildComment},

	{Path: "/api/v1/jobs", Method: "GET", Name: ListAllJobs},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs", Method: "GET", Name: ListJobs},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name", Method: "GET", Name: GetJob},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds", Method: "GET", Name: ListJobBuilds},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds", Method: "POST", Name: CreateJobBuild},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds/:build_name", Method: "POST", Name: RerunJobBuild},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/inputs", Method: "GET", Name: ListJobInputs},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds/:build_name", Method: "GET", Name: GetJobBuild},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/pause", Method: "PUT", Name: PauseJob},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/unpause", Method: "PUT", Name: UnpauseJob},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/schedule", Method: "PUT", Name: ScheduleJob},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/badge", Method: "GET", Name: JobBadge},
	{Path: "/api/v1/pipelines/:pipeline_name/jobs/:job_name/badge", Method: "GET", Name: MainJobBadge},

	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/tasks/:step_name/cache", Method: "DELETE", Name: ClearTaskCache},

	{Path: "/api/v1/pipelines", Method: "GET", Name: ListAllPipelines},
	{Path: "/api/v1/teams/:team_name/pipelines", Method: "GET", Name: ListPipelines},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name", Method: "GET", Name: GetPipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name", Method: "DELETE", Name: DeletePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/ordering", Method: "PUT", Name: OrderPipelines},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/ordering", Method: "PUT", Name: OrderPipelinesWithinGroup},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/pause", Method: "PUT", Name: PausePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/archive", Method: "PUT", Name: ArchivePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/unpause", Method: "PUT", Name: UnpausePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/expose", Method: "PUT", Name: ExposePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/hide", Method: "PUT", Name: HidePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/versions-db", Method: "GET", Name: GetVersionsDB},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/rename", Method: "PUT", Name: RenamePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/builds", Method: "GET", Name: ListPipelineBuilds},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/builds", Method: "POST", Name: CreatePipelineBuild},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/badge", Method: "GET", Name: PipelineBadge},

	{Path: "/api/v1/resources", Method: "GET", Name: ListAllResources},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources", Method: "GET", Name: ListResources},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resource-types", Method: "GET", Name: ListResourceTypes},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name", Method: "GET", Name: GetResource},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/check", Method: "POST", Name: CheckResource},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/check/webhook", Method: "POST", Name: CheckResourceWebHook},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resource-types/:resource_type_name/check", Method: "POST", Name: CheckResourceType},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/prototypes/:prototype_name/check", Method: "POST", Name: CheckPrototype},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/cache", Method: "DELETE", Name: ClearResourceCache},

	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions", Method: "GET", Name: ListResourceVersions},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_config_version_id", Method: "GET", Name: GetResourceVersion},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_config_version_id/enable", Method: "PUT", Name: EnableResourceVersion},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_config_version_id/disable", Method: "PUT", Name: DisableResourceVersion},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_config_version_id/pin", Method: "PUT", Name: PinResourceVersion},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/unpin", Method: "PUT", Name: UnpinResource},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/pin_comment", Method: "PUT", Name: SetPinCommentOnResource},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_config_version_id/input_to", Method: "GET", Name: ListBuildsWithVersionAsInput},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_config_version_id/output_of", Method: "GET", Name: ListBuildsWithVersionAsOutput},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_config_version_id/downstream", Method: "GET", Name: GetDownstreamResourceCausality},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_config_version_id/upstream", Method: "GET", Name: GetUpstreamResourceCausality},
	// {Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/causality", Method: "GET", Name: GetResourceCausality},

	{Path: "/api/v1/teams/:team_name/cc.xml", Method: "GET", Name: GetCC},

	{Path: "/api/v1/workers", Method: "GET", Name: ListWorkers},
	{Path: "/api/v1/workers", Method: "POST", Name: RegisterWorker},
	{Path: "/api/v1/workers/:worker_name/land", Method: "PUT", Name: LandWorker},
	{Path: "/api/v1/workers/:worker_name/retire", Method: "PUT", Name: RetireWorker},
	{Path: "/api/v1/workers/:worker_name/prune", Method: "PUT", Name: PruneWorker},
	{Path: "/api/v1/workers/:worker_name/heartbeat", Method: "PUT", Name: HeartbeatWorker},
	{Path: "/api/v1/workers/:worker_name", Method: "DELETE", Name: DeleteWorker},

	{Path: "/api/v1/log-level", Method: "GET", Name: GetLogLevel},
	{Path: "/api/v1/log-level", Method: "PUT", Name: SetLogLevel},

	{Path: "/api/v1/cli", Method: "GET", Name: DownloadCLI},
	{Path: "/api/v1/info", Method: "GET", Name: GetInfo},
	{Path: "/api/v1/info/creds", Method: "GET", Name: GetInfoCreds},

	{Path: "/api/v1/user", Method: "GET", Name: GetUser},
	{Path: "/api/v1/users", Method: "GET", Name: ListActiveUsersSince},

	{Path: "/api/v1/containers/destroying", Method: "GET", Name: ListDestroyingContainers},
	{Path: "/api/v1/containers/report", Method: "PUT", Name: ReportWorkerContainers},
	{Path: "/api/v1/teams/:team_name/containers", Method: "GET", Name: ListContainers},
	{Path: "/api/v1/teams/:team_name/containers/:id", Method: "GET", Name: GetContainer},
	{Path: "/api/v1/teams/:team_name/containers/:id/hijack", Method: "GET", Name: HijackContainer},

	{Path: "/api/v1/teams/:team_name/volumes", Method: "GET", Name: ListVolumes},
	{Path: "/api/v1/volumes/destroying", Method: "GET", Name: ListDestroyingVolumes},
	{Path: "/api/v1/volumes/report", Method: "PUT", Name: ReportWorkerVolumes},

	{Path: "/api/v1/teams", Method: "GET", Name: ListTeams},
	{Path: "/api/v1/teams/:team_name", Method: "GET", Name: GetTeam},
	{Path: "/api/v1/teams/:team_name", Method: "PUT", Name: SetTeam},
	{Path: "/api/v1/teams/:team_name/rename", Method: "PUT", Name: RenameTeam},
	{Path: "/api/v1/teams/:team_name", Method: "DELETE", Name: DestroyTeam},
	{Path: "/api/v1/teams/:team_name/builds", Method: "GET", Name: ListTeamBuilds},

	{Path: "/api/v1/teams/:team_name/artifacts", Method: "POST", Name: CreateArtifact},
	{Path: "/api/v1/teams/:team_name/artifacts/:artifact_id", Method: "GET", Name: GetArtifact},

	{Path: "/api/v1/wall", Method: "GET", Name: GetWall},
	{Path: "/api/v1/wall", Method: "PUT", Name: SetWall},
	{Path: "/api/v1/wall", Method: "DELETE", Name: ClearWall},
})
