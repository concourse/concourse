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

	GetJob         = "GetJob"
	CreateJobBuild = "CreateJobBuild"
	ListJobs       = "ListJobs"
	ListJobBuilds  = "ListJobBuilds"
	ListJobInputs  = "ListJobInputs"
	GetJobBuild    = "GetJobBuild"
	PauseJob       = "PauseJob"
	UnpauseJob     = "UnpauseJob"
	GetVersionsDB  = "GetVersionsDB"
	JobBadge       = "JobBadge"
	MainJobBadge   = "MainJobBadge"

	ListResources        = "ListResources"
	GetResource          = "GetResource"
	PauseResource        = "PauseResource"
	UnpauseResource      = "UnpauseResource"
	CheckResource        = "CheckResource"
	CheckResourceWebHook = "CheckResourceWebHook"

	ListResourceVersions          = "ListResourceVersions"
	GetResourceVersion            = "GetResourceVersion"
	EnableResourceVersion         = "EnableResourceVersion"
	DisableResourceVersion        = "DisableResourceVersion"
	ListBuildsWithVersionAsInput  = "ListBuildsWithVersionAsInput"
	ListBuildsWithVersionAsOutput = "ListBuildsWithVersionAsOutput"
	GetResourceCausality          = "GetResourceCausality"

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
	CreatePipelineBuild = "CreatePipelineBuild"
	PipelineBadge       = "PipelineBadge"

	CreatePipe = "CreatePipe"
	WritePipe  = "WritePipe"
	ReadPipe   = "ReadPipe"

	RegisterWorker  = "RegisterWorker"
	LandWorker      = "LandWorker"
	RetireWorker    = "RetireWorker"
	PruneWorker     = "PruneWorker"
	HeartbeatWorker = "HeartbeatWorker"
	ListWorkers     = "ListWorkers"
	DeleteWorker    = "DeleteWorker"

	SetLogLevel = "SetLogLevel"
	GetLogLevel = "GetLogLevel"

	DownloadCLI = "DownloadCLI"
	GetInfo     = "Info"

	ListContainers  = "ListContainers"
	GetContainer    = "GetContainer"
	HijackContainer = "HijackContainer"

	ListVolumes = "ListVolumes"

	LegacyListAuthMethods = "LegacyListAuthMethods"
	LegacyGetAuthToken    = "LegacyGetAuthToken"
	LegacyGetUser         = "LegacyGetUser"

	ListTeams   = "ListTeams"
	SetTeam     = "SetTeam"
	RenameTeam  = "RenameTeam"
	DestroyTeam = "DestroyTeam"
)

var Routes = rata.Routes([]rata.Route{
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/config", Method: "PUT", Name: SaveConfig},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/config", Method: "GET", Name: GetConfig},

	{Path: "/api/v1/builds", Method: "POST", Name: CreateBuild},
	{Path: "/api/v1/builds", Method: "GET", Name: ListBuilds},
	{Path: "/api/v1/builds/:build_id", Method: "GET", Name: GetBuild},
	{Path: "/api/v1/builds/:build_id/plan", Method: "GET", Name: GetBuildPlan},
	{Path: "/api/v1/builds/:build_id/events", Method: "GET", Name: BuildEvents},
	{Path: "/api/v1/builds/:build_id/resources", Method: "GET", Name: BuildResources},
	{Path: "/api/v1/builds/:build_id/abort", Method: "PUT", Name: AbortBuild},
	{Path: "/api/v1/builds/:build_id/preparation", Method: "GET", Name: GetBuildPreparation},

	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs", Method: "GET", Name: ListJobs},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name", Method: "GET", Name: GetJob},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds", Method: "GET", Name: ListJobBuilds},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds", Method: "POST", Name: CreateJobBuild},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/inputs", Method: "GET", Name: ListJobInputs},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds/:build_name", Method: "GET", Name: GetJobBuild},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/pause", Method: "PUT", Name: PauseJob},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/unpause", Method: "PUT", Name: UnpauseJob},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/badge", Method: "GET", Name: JobBadge},
	{Path: "/api/v1/pipelines/:pipeline_name/jobs/:job_name/badge", Method: "GET", Name: MainJobBadge},

	{Path: "/api/v1/pipelines", Method: "GET", Name: ListAllPipelines},
	{Path: "/api/v1/teams/:team_name/pipelines", Method: "GET", Name: ListPipelines},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name", Method: "GET", Name: GetPipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name", Method: "DELETE", Name: DeletePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/ordering", Method: "PUT", Name: OrderPipelines},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/pause", Method: "PUT", Name: PausePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/unpause", Method: "PUT", Name: UnpausePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/expose", Method: "PUT", Name: ExposePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/hide", Method: "PUT", Name: HidePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/versions-db", Method: "GET", Name: GetVersionsDB},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/rename", Method: "PUT", Name: RenamePipeline},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/builds", Method: "POST", Name: CreatePipelineBuild},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/badge", Method: "GET", Name: PipelineBadge},

	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources", Method: "GET", Name: ListResources},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name", Method: "GET", Name: GetResource},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/pause", Method: "PUT", Name: PauseResource},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/unpause", Method: "PUT", Name: UnpauseResource},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/check", Method: "POST", Name: CheckResource},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/check/webhook", Method: "POST", Name: CheckResourceWebHook},

	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions", Method: "GET", Name: ListResourceVersions},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_version_id", Method: "GET", Name: GetResourceVersion},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_version_id/enable", Method: "PUT", Name: EnableResourceVersion},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_version_id/disable", Method: "PUT", Name: DisableResourceVersion},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_version_id/input_to", Method: "GET", Name: ListBuildsWithVersionAsInput},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_version_id/output_of", Method: "GET", Name: ListBuildsWithVersionAsOutput},
	{Path: "/api/v1/teams/:team_name/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_version_id/causality", Method: "GET", Name: GetResourceCausality},

	{Path: "/api/v1/pipes", Method: "POST", Name: CreatePipe},
	{Path: "/api/v1/pipes/:pipe_id", Method: "PUT", Name: WritePipe},
	{Path: "/api/v1/pipes/:pipe_id", Method: "GET", Name: ReadPipe},

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

	{Path: "/api/v1/containers", Method: "GET", Name: ListContainers},
	{Path: "/api/v1/containers/:id", Method: "GET", Name: GetContainer},
	{Path: "/api/v1/containers/:id/hijack", Method: "GET", Name: HijackContainer},

	{Path: "/api/v1/volumes", Method: "GET", Name: ListVolumes},

	{Path: "/api/v1/teams/:team_name/auth/methods", Method: "GET", Name: LegacyListAuthMethods},
	{Path: "/api/v1/teams/:team_name/auth/token", Method: "GET", Name: LegacyGetAuthToken},
	{Path: "/api/v1/user", Method: "GET", Name: LegacyGetUser},

	{Path: "/api/v1/teams", Method: "GET", Name: ListTeams},
	{Path: "/api/v1/teams/:team_name", Method: "PUT", Name: SetTeam},
	{Path: "/api/v1/teams/:team_name/rename", Method: "PUT", Name: RenameTeam},
	{Path: "/api/v1/teams/:team_name", Method: "DELETE", Name: DestroyTeam},
})
