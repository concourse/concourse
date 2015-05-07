package atc

import "github.com/tedsuo/rata"

const (
	SaveConfig = "SaveConfig"
	GetConfig  = "GetConfig"

	Hijack = "Hijack"

	CreateBuild = "CreateBuild"
	ListBuilds  = "ListBuilds"
	BuildEvents = "BuildEvents"
	AbortBuild  = "AbortBuild"

	GetJob        = "GetJob"
	ListJobs      = "ListJobs"
	ListJobBuilds = "ListJobBuilds"
	GetJobBuild   = "GetJobBuild"
	PauseJob      = "PauseJob"
	UnpauseJob    = "UnpauseJob"

	ListResources          = "ListResources"
	EnableResourceVersion  = "EnableResourceVersion"
	DisableResourceVersion = "DisableResourceVersion"
	PauseResource          = "PauseResource"
	UnpauseResource        = "UnpauseResource"

	ListPipelines   = "ListPipelines"
	DeletePipeline  = "DeletePipeline"
	OrderPipelines  = "OrderPipelines"
	PausePipeline   = "PausePipeline"
	UnpausePipeline = "UnpausePipeline"

	CreatePipe = "CreatePipe"
	WritePipe  = "WritePipe"
	ReadPipe   = "ReadPipe"

	RegisterWorker = "RegisterWorker"
	ListWorkers    = "ListWorkers"

	SetLogLevel = "SetLogLevel"
	GetLogLevel = "GetLogLevel"

	DownloadCLI = "DownloadCLI"
)

var Routes = rata.Routes{
	{Path: "/api/v1/pipelines/:pipeline_name/config", Method: "PUT", Name: SaveConfig},
	{Path: "/api/v1/pipelines/:pipeline_name/config", Method: "GET", Name: GetConfig},

	{Path: "/api/v1/builds", Method: "POST", Name: CreateBuild},
	{Path: "/api/v1/builds", Method: "GET", Name: ListBuilds},
	{Path: "/api/v1/builds/:build_id/events", Method: "GET", Name: BuildEvents},
	{Path: "/api/v1/builds/:build_id/abort", Method: "POST", Name: AbortBuild},
	{Path: "/api/v1/hijack", Method: "POST", Name: Hijack},

	{Path: "/api/v1/pipelines/:pipeline_name/jobs", Method: "GET", Name: ListJobs},
	{Path: "/api/v1/pipelines/:pipeline_name/jobs/:job_name", Method: "GET", Name: GetJob},
	{Path: "/api/v1/pipelines/:pipeline_name/jobs/:job_name/builds", Method: "GET", Name: ListJobBuilds},
	{Path: "/api/v1/pipelines/:pipeline_name/jobs/:job_name/builds/:build_name", Method: "GET", Name: GetJobBuild},
	{Path: "/api/v1/pipelines/:pipeline_name/jobs/:job_name/pause", Method: "PUT", Name: PauseJob},
	{Path: "/api/v1/pipelines/:pipeline_name/jobs/:job_name/unpause", Method: "PUT", Name: UnpauseJob},

	{Path: "/api/v1/pipelines", Method: "GET", Name: ListPipelines},
	{Path: "/api/v1/pipelines/:pipeline_name", Method: "DELETE", Name: DeletePipeline},
	{Path: "/api/v1/pipelines/ordering", Method: "PUT", Name: OrderPipelines},
	{Path: "/api/v1/pipelines/:pipeline_name/pause", Method: "PUT", Name: PausePipeline},
	{Path: "/api/v1/pipelines/:pipeline_name/unpause", Method: "PUT", Name: UnpausePipeline},

	{Path: "/api/v1/pipelines/:pipeline_name/resources", Method: "GET", Name: ListResources},
	{Path: "/api/v1/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_version_id/enable", Method: "PUT", Name: EnableResourceVersion},
	{Path: "/api/v1/pipelines/:pipeline_name/resources/:resource_name/versions/:resource_version_id/disable", Method: "PUT", Name: DisableResourceVersion},
	{Path: "/api/v1/pipelines/:pipeline_name/resources/:resource_name/pause", Method: "PUT", Name: PauseResource},
	{Path: "/api/v1/pipelines/:pipeline_name/resources/:resource_name/unpause", Method: "PUT", Name: UnpauseResource},

	{Path: "/api/v1/pipes", Method: "POST", Name: CreatePipe},
	{Path: "/api/v1/pipes/:pipe_id", Method: "PUT", Name: WritePipe},
	{Path: "/api/v1/pipes/:pipe_id", Method: "GET", Name: ReadPipe},

	{Path: "/api/v1/workers", Method: "GET", Name: ListWorkers},
	{Path: "/api/v1/workers", Method: "POST", Name: RegisterWorker},

	{Path: "/api/v1/log-level", Method: "GET", Name: GetLogLevel},
	{Path: "/api/v1/log-level", Method: "PUT", Name: SetLogLevel},

	{Path: "/api/v1/cli", Method: "GET", Name: DownloadCLI},
}
