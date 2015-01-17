package atc

import "github.com/tedsuo/rata"

const (
	SaveConfig = "SaveConfig"
	GetConfig  = "GetConfig"

	CreateBuild = "CreateBuild"
	ListBuilds  = "ListBuilds"
	BuildEvents = "BuildEvents"
	AbortBuild  = "AbortBuild"
	HijackBuild = "HijackBuild"

	GetJob        = "GetJob"
	ListJobs      = "ListJobs"
	ListJobBuilds = "ListJobBuilds"
	GetJobBuild   = "GetJobBuild"

	ListResources          = "ListResources"
	EnableResourceVersion  = "EnableResourceVersion"
	DisableResourceVersion = "DisableResourceVersion"

	CreatePipe = "CreatePipe"
	WritePipe  = "WritePipe"
	ReadPipe   = "ReadPipe"

	RegisterWorker = "RegisterWorker"
	ListWorkers    = "ListWorkers"
)

var Routes = rata.Routes{
	{Path: "/api/v1/config", Method: "PUT", Name: SaveConfig},
	{Path: "/api/v1/config", Method: "GET", Name: GetConfig},

	{Path: "/api/v1/builds", Method: "POST", Name: CreateBuild},
	{Path: "/api/v1/builds", Method: "GET", Name: ListBuilds},
	{Path: "/api/v1/builds/:build_id/events", Method: "GET", Name: BuildEvents},
	{Path: "/api/v1/builds/:build_id/abort", Method: "POST", Name: AbortBuild},
	{Path: "/api/v1/builds/:build_id/hijack", Method: "POST", Name: HijackBuild},

	{Path: "/api/v1/jobs", Method: "GET", Name: ListJobs},
	{Path: "/api/v1/jobs/:job_name", Method: "GET", Name: GetJob},
	{Path: "/api/v1/jobs/:job_name/builds", Method: "GET", Name: ListJobBuilds},
	{Path: "/api/v1/jobs/:job_name/builds/:build_name", Method: "GET", Name: GetJobBuild},

	{Path: "/api/v1/resources", Method: "GET", Name: ListResources},
	{Path: "/api/v1/resources/:version_id/enable", Method: "PUT", Name: EnableResourceVersion},
	{Path: "/api/v1/resources/:version_id/disable", Method: "PUT", Name: DisableResourceVersion},

	{Path: "/api/v1/pipes", Method: "POST", Name: CreatePipe},
	{Path: "/api/v1/pipes/:pipe_id", Method: "PUT", Name: WritePipe},
	{Path: "/api/v1/pipes/:pipe_id", Method: "GET", Name: ReadPipe},

	{Path: "/api/v1/workers", Method: "POST", Name: RegisterWorker},
	{Path: "/api/v1/workers", Method: "GET", Name: ListWorkers},
}
