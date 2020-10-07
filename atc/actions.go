package atc

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
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
	ArchivePipeline     = "ArchivePipeline"
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

func GetParam(r *http.Request, paramName string) string {
	routeParam := mux.Vars(r)[strings.TrimPrefix(paramName, ":")]
	if routeParam == "" {
		return r.FormValue(paramName)
	} else {
		return routeParam
	}
}
