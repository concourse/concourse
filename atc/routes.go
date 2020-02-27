package atc

import "github.com/tedsuo/rata"

const (
	Get    = "GET"
	Post   = "POST"
	Put    = "PUT"
	Delete = "DELETE"
)

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

	GetCheck = "GetCheck"

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

	ListResourceVersions          = "ListResourceVersions"
	GetResourceVersion            = "GetResourceVersion"
	EnableResourceVersion         = "EnableResourceVersion"
	DisableResourceVersion        = "DisableResourceVersion"
	PinResourceVersion            = "PinResourceVersion"
	UnpinResource                 = "UnpinResource"
	SetPinCommentOnResource       = "SetPinCommentOnResource"
	ListBuildsWithVersionAsInput  = "ListBuildsWithVersionAsInput"
	ListBuildsWithVersionAsOutput = "ListBuildsWithVersionAsOutput"
	GetResourceCausality          = "GetResourceCausality"

	GetCC = "GetCC"

	ListAllPipelines    = "ListAllPipelines"
	ListPipelines       = "ListPipelines"
	GetPipeline         = "GetPipeline"
	DeletePipeline      = "DeletePipeline"
	OrderPipelines      = "OrderPipelines"
	PausePipeline       = "PausePipeline"
	UnpausePipeline     = "UnpausePipeline"
	ExposePipeline      = "ExposePipeline"
	HidePipeline        = "HidePipeline"
	RenamePipeline      = "RenamePipeline"
	ListPipelineBuilds  = "ListPipelineBuilds"
	CreatePipelineBuild = "CreatePipelineBuild"
	PipelineBadge       = "PipelineBadge"

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
	GetInfo      = "Info"
	GetInfoCreds = "InfoCreds"

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
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/config", Method: Put, Name: SaveConfig},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/config", Method: Get, Name: GetConfig},

	{Path: "/api/v1/teams/:team_name/builds", Method: Post, Name: CreateBuild},

	{Path: "/api/v1/builds", Method: Get, Name: ListBuilds},
	{Path: "/api/v1/builds/:build_id", Method: Get, Name: GetBuild},
	{Path: "/api/v1/builds/:build_id/plan", Method: Get, Name: GetBuildPlan},
	{Path: "/api/v1/builds/:build_id/events", Method: Get, Name: BuildEvents},
	{Path: "/api/v1/builds/:build_id/resources", Method: Get, Name: BuildResources},
	{Path: "/api/v1/builds/:build_id/abort", Method: Put, Name: AbortBuild},
	{Path: "/api/v1/builds/:build_id/preparation", Method: Get, Name: GetBuildPreparation},
	{Path: "/api/v1/builds/:build_id/artifacts", Method: Get, Name: ListBuildArtifacts},

	{Path: "/api/v1/checks/:check_id", Method: Get, Name: GetCheck},

	{Path: "/api/v1/jobs", Method: Get, Name: ListAllJobs},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs", Method: Get, Name: ListJobs},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name", Method: Get, Name: GetJob},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds", Method: Get, Name: ListJobBuilds},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds", Method: Post, Name: CreateJobBuild},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds/:build_name", Method: Post, Name: RerunJobBuild},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/inputs", Method: Get, Name: ListJobInputs},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds/:build_name", Method: Get, Name: GetJobBuild},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/pause", Method: Put, Name: PauseJob},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/unpause", Method: Put, Name: UnpauseJob},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/schedule", Method: Put, Name: ScheduleJob},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/badge", Method: Get, Name: JobBadge},
	{Path: "/api/v1/pipelines/:pipeline_name/jobs/:job_name/badge", Method: Get, Name: MainJobBadge},

	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/tasks/:step_name/cache", Method: Delete, Name: ClearTaskCache},

	{Path: "/api/v1/pipelines", Method: Get, Name: ListAllPipelines},
	{Path: "/api/v1/teams/:team_name/pipelines", Method: Get, Name: ListPipelines},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name", Method: Get, Name: GetPipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name", Method: Delete, Name: DeletePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/ordering", Method: Put, Name: OrderPipelines},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/pause", Method: Put, Name: PausePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/unpause", Method: Put, Name: UnpausePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/expose", Method: Put, Name: ExposePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/hide", Method: Put, Name: HidePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/versions-db", Method: Get, Name: GetVersionsDB},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/rename", Method: Put, Name: RenamePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/builds", Method: Get, Name: ListPipelineBuilds},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/builds", Method: Post, Name: CreatePipelineBuild},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/badge", Method: Get, Name: PipelineBadge},

	{Path: "/api/v1/resources", Method: Get, Name: ListAllResources},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources", Method: Get, Name: ListResources},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resource-types", Method: Get, Name: ListResourceTypes},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name", Method: Get, Name: GetResource},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/check", Method: Post, Name: CheckResource},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/check/webhook", Method: Post, Name: CheckResourceWebHook},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resource-types/:resource_type_name/check", Method: Post, Name: CheckResourceType},

	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions", Method: Get, Name: ListResourceVersions},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_config_version_id", Method: Get, Name: GetResourceVersion},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_config_version_id/enable", Method: Put, Name: EnableResourceVersion},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_config_version_id/disable", Method: Put, Name: DisableResourceVersion},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_config_version_id/pin", Method: Put, Name: PinResourceVersion},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/unpin", Method: Put, Name: UnpinResource},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/pin_comment", Method: Put, Name: SetPinCommentOnResource},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_config_version_id/input_to", Method: Get, Name: ListBuildsWithVersionAsInput},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_config_version_id/output_of", Method: Get, Name: ListBuildsWithVersionAsOutput},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_version_id/causality", Method: Get, Name: GetResourceCausality},

	{Path: "/api/v1/teams/:team_name/cc.xml", Method: Get, Name: GetCC},

	{Path: "/api/v1/workers", Method: Get, Name: ListWorkers},
	{Path: "/api/v1/workers", Method: Post, Name: RegisterWorker},
	{Path: "/api/v1/workers/:worker_name/land", Method: Put, Name: LandWorker},
	{Path: "/api/v1/workers/:worker_name/retire", Method: Put, Name: RetireWorker},
	{Path: "/api/v1/workers/:worker_name/prune", Method: Put, Name: PruneWorker},
	{Path: "/api/v1/workers/:worker_name/heartbeat", Method: Put, Name: HeartbeatWorker},
	{Path: "/api/v1/workers/:worker_name", Method: Delete, Name: DeleteWorker},

	{Path: "/api/v1/log-level", Method: Get, Name: GetLogLevel},
	{Path: "/api/v1/log-level", Method: Put, Name: SetLogLevel},

	{Path: "/api/v1/cli", Method: Get, Name: DownloadCLI},
	{Path: "/api/v1/info", Method: Get, Name: GetInfo},
	{Path: "/api/v1/info/creds", Method: Get, Name: GetInfoCreds},

	{Path: "/api/v1/users", Method: Get, Name: ListActiveUsersSince},

	{Path: "/api/v1/containers/destroying", Method: Get, Name: ListDestroyingContainers},
	{Path: "/api/v1/containers/report", Method: Put, Name: ReportWorkerContainers},
	{Path: "/api/v1/teams/:team_name/containers", Method: Get, Name: ListContainers},
	{Path: "/api/v1/teams/:team_name/containers/:id", Method: Get, Name: GetContainer},
	{Path: "/api/v1/teams/:team_name/containers/:id/hijack", Method: Get, Name: HijackContainer},

	{Path: "/api/v1/teams/:team_name/volumes", Method: Get, Name: ListVolumes},
	{Path: "/api/v1/volumes/destroying", Method: Get, Name: ListDestroyingVolumes},
	{Path: "/api/v1/volumes/report", Method: Put, Name: ReportWorkerVolumes},

	{Path: "/api/v1/teams", Method: Get, Name: ListTeams},
	{Path: "/api/v1/teams/:team_name", Method: Get, Name: GetTeam},
	{Path: "/api/v1/teams/:team_name", Method: Put, Name: SetTeam},
	{Path: "/api/v1/teams/:team_name/rename", Method: Put, Name: RenameTeam},
	{Path: "/api/v1/teams/:team_name", Method: Delete, Name: DestroyTeam},
	{Path: "/api/v1/teams/:team_name/builds", Method: Get, Name: ListTeamBuilds},

	{Path: "/api/v1/teams/:team_name/artifacts", Method: Post, Name: CreateArtifact},
	{Path: "/api/v1/teams/:team_name/artifacts/:artifact_id", Method: Get, Name: GetArtifact},

	{Path: "/api/v1/wall", Method: Get, Name: GetWall},
	{Path: "/api/v1/wall", Method: Put, Name: SetWall},
	{Path: "/api/v1/wall", Method: Delete, Name: ClearWall},
})
