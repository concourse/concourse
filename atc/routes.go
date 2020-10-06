package atc

import (
	"io"
	"net/http"
	"net/url"
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

func router() *mux.Router {
	r := mux.NewRouter()

	r.Name(SaveConfig).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/config").Methods("PUT")
	r.Name(GetConfig).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/config").Methods("GET")

	r.Name(CreateBuild).Path("/api/v1/teams/{team_name}/builds").Methods("POST")

	r.Name(ListBuilds).Path("/api/v1/builds").Methods("GET")
	r.Name(GetBuild).Path("/api/v1/builds/{build_id}").Methods("GET")
	r.Name(GetBuildPlan).Path("/api/v1/builds/{build_id}/plan").Methods("GET")
	r.Name(BuildEvents).Path("/api/v1/builds/{build_id}/events").Methods("GET")
	r.Name(BuildResources).Path("/api/v1/builds/{build_id}/resources").Methods("GET")
	r.Name(AbortBuild).Path("/api/v1/builds/{build_id}/abort").Methods("PUT")
	r.Name(GetBuildPreparation).Path("/api/v1/builds/{build_id}/preparation").Methods("GET")
	r.Name(ListBuildArtifacts).Path("/api/v1/builds/{build_id}/artifacts").Methods("GET")

	r.Name(ListAllJobs).Path("/api/v1/jobs").Methods("GET")
	r.Name(ListJobs).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs").
		Methods("GET")
	r.Name(GetJob).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}").
		Methods("GET")
	r.Name(ListJobBuilds).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/builds").
		Methods("GET")
	r.Name(CreateJobBuild).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/builds").
		Methods("POST")
	r.Name(RerunJobBuild).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/builds/{build_name}").
		Methods("POST")
	r.Name(ListJobInputs).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/inputs").
		Methods("GET")
	r.Name(GetJobBuild).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/builds/{build_name}").
		Methods("GET")
	r.Name(PauseJob).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/pause").
		Methods("PUT")
	r.Name(UnpauseJob).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/unpause").
		Methods("PUT")
	r.Name(ScheduleJob).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/schedule").
		Methods("PUT")
	r.Name(JobBadge).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/badge").
		Methods("GET")
	r.Name(MainJobBadge).Path("/api/v1/pipelines/{pipeline_name}/jobs/{job_name}/badge").Methods("GET")

	r.Name(ClearTaskCache).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/tasks/{step_name}/cache").
		Methods("DELETE")

	r.Name(ListAllPipelines).Path("/api/v1/pipelines").Methods("GET")
	r.Name(ListPipelines).Path("/api/v1/teams/{team_name}/pipelines").Methods("GET")
	r.Name(GetPipeline).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}").Methods("GET")
	r.Name(DeletePipeline).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}").Methods("DELETE")
	r.Name(OrderPipelines).Path("/api/v1/teams/{team_name}/pipelines/ordering").Methods("PUT")
	r.Name(PausePipeline).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/pause").Methods("PUT")
	r.Name(ArchivePipeline).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/archive").Methods("PUT")
	r.Name(UnpausePipeline).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/unpause").Methods("PUT")
	r.Name(ExposePipeline).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/expose").Methods("PUT")
	r.Name(HidePipeline).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/hide").Methods("PUT")
	r.Name(GetVersionsDB).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/versions-db").Methods("GET")
	r.Name(RenamePipeline).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/rename").Methods("PUT")
	r.Name(ListPipelineBuilds).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/builds").Methods("GET")
	r.Name(CreatePipelineBuild).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/builds").Methods("POST")
	r.Name(PipelineBadge).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/badge").Methods("GET")

	r.Name(ListAllResources).Path("/api/v1/resources").Methods("GET")
	r.Name(ListResources).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources").
		Methods("GET")
	r.Name(ListResourceTypes).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resource-types").
		Methods("GET")
	r.Name(GetResource).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}").
		Methods("GET")
	r.Name(CheckResource).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/check").
		Methods("POST")
	r.Name(CheckResourceWebHook).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/check/webhook").
		Methods("POST")
	r.Name(CheckResourceType).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resource-types/{resource_type_name}/check").
		Methods("POST")

	r.Name(ListResourceVersions).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/versions").
		Methods("GET")
	r.Name(GetResourceVersion).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/versions/{resource_config_version_id}").
		Methods("GET")
	r.Name(EnableResourceVersion).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/versions/{resource_config_version_id}/enable").
		Methods("PUT")
	r.Name(DisableResourceVersion).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/versions/{resource_config_version_id}/disable").
		Methods("PUT")
	r.Name(PinResourceVersion).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/versions/{resource_config_version_id}/pin").
		Methods("PUT")
	r.Name(UnpinResource).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/unpin").Methods("PUT")
	r.Name(SetPinCommentOnResource).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/pin_comment").Methods("PUT")
	r.Name(ListBuildsWithVersionAsInput).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/versions/{resource_config_version_id}/input_to").
		Methods("GET")
	r.Name(ListBuildsWithVersionAsOutput).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/versions/{resource_config_version_id}/output_of").
		Methods("GET")
	r.Name(GetResourceCausality).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/versions/{resource_version_id}/causality").
		Methods("GET")

	r.Name(GetCC).Path("/api/v1/teams/{team_name}/cc.xml").Methods("GET")

	r.Name(ListWorkers).Path("/api/v1/workers").Methods("GET")
	r.Name(RegisterWorker).Path("/api/v1/workers").Methods("POST")
	r.Name(LandWorker).Path("/api/v1/workers/{worker_name}/land").Methods("PUT")
	r.Name(RetireWorker).Path("/api/v1/workers/{worker_name}/retire").Methods("PUT")
	r.Name(PruneWorker).Path("/api/v1/workers/{worker_name}/prune").Methods("PUT")
	r.Name(HeartbeatWorker).Path("/api/v1/workers/{worker_name}/heartbeat").Methods("PUT")
	r.Name(DeleteWorker).Path("/api/v1/workers/{worker_name}").Methods("DELETE")

	r.Name(GetLogLevel).Path("/api/v1/log-level").Methods("GET")
	r.Name(SetLogLevel).Path("/api/v1/log-level").Methods("PUT")

	r.Name(DownloadCLI).Path("/api/v1/cli").Methods("GET")
	r.Name(GetInfo).Path("/api/v1/info").Methods("GET")
	r.Name(GetInfoCreds).Path("/api/v1/info/creds").Methods("GET")

	r.Name(GetUser).Path("/api/v1/user").Methods("GET")
	r.Name(ListActiveUsersSince).Path("/api/v1/users").Methods("GET")

	r.Name(ListDestroyingContainers).Path("/api/v1/containers/destroying").Methods("GET")
	r.Name(ReportWorkerContainers).Path("/api/v1/containers/report").Methods("PUT")
	r.Name(ListContainers).Path("/api/v1/teams/{team_name}/containers").Methods("GET")
	r.Name(GetContainer).Path("/api/v1/teams/{team_name}/containers/{id}").Methods("GET")
	r.Name(HijackContainer).Path("/api/v1/teams/{team_name}/containers/{id}/hijack").Methods("GET")

	r.Name(ListVolumes).Path("/api/v1/teams/{team_name}/volumes").Methods("GET")
	r.Name(ListDestroyingVolumes).Path("/api/v1/volumes/destroying").Methods("GET")
	r.Name(ReportWorkerVolumes).Path("/api/v1/volumes/report").Methods("PUT")

	r.Name(ListTeams).Path("/api/v1/teams").Methods("GET")
	r.Name(GetTeam).Path("/api/v1/teams/{team_name}").Methods("GET")
	r.Name(SetTeam).Path("/api/v1/teams/{team_name}").Methods("PUT")
	r.Name(RenameTeam).Path("/api/v1/teams/{team_name}/rename").Methods("PUT")
	r.Name(DestroyTeam).Path("/api/v1/teams/{team_name}").Methods("DELETE")
	r.Name(ListTeamBuilds).Path("/api/v1/teams/{team_name}/builds").Methods("GET")

	r.Name(CreateArtifact).Path("/api/v1/teams/{team_name}/artifacts").Methods("POST")
	r.Name(GetArtifact).Path("/api/v1/teams/{team_name}/artifacts/{artifact_id}").Methods("GET")

	r.Name(GetWall).Path("/api/v1/wall").Methods("GET")
	r.Name(SetWall).Path("/api/v1/wall").Methods("PUT")
	r.Name(ClearWall).Path("/api/v1/wall").Methods("DELETE")

	return r
}

func RouteNames() []string {
	names := []string{}
	router().Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		names = append(names, route.GetName())
		return nil
	})
	return names
}

type Endpoint interface {
	CreateRequest(string, map[string]string, io.Reader) (*http.Request, error)
}

func NewEndpoint(host string) Endpoint {
	return &endpoint{
		host: host,
	}
}

type endpoint struct {
	host string
}

func (rae *endpoint) CreateRequest(
	action string,
	params map[string]string,
	body io.Reader,
) (*http.Request, error) {
	hostURL, err := url.Parse(rae.host)
	if err != nil {
		return &http.Request{}, err
	}
	r := router()
	route := r.Get(action).Host(hostURL.Host)
	pairs := []string{}
	for key, val := range params {
		pairs = append(pairs, key, val)
	}
	url, err := route.URL(pairs...)
	if err != nil {
		return &http.Request{}, err
	}
	methods, err := route.GetMethods()
	if err != nil {
		return &http.Request{}, err
	}
	return http.NewRequest(methods[0], url.String(), body)
}

func CreatePathForRoute(action string, params map[string]string) (string, error) {
	r := router()
	route := r.Get(action)
	pairs := []string{}
	for key, val := range params {
		pairs = append(pairs, key, val)
	}
	path, err := route.URLPath(pairs...)
	if err != nil {
		return "", err
	}
	return path.String(), nil
}

func NewRouter(handlers map[string]http.Handler) (http.Handler, error) {
	r := router()
	for action, handler := range handlers {
		r.Get(action).Handler(handler)
	}
	return r, nil
}

func GetParam(r *http.Request, paramName string) string {
	routeParam := mux.Vars(r)[strings.TrimPrefix(paramName, ":")]
	if routeParam == "" {
		return r.FormValue(paramName)
	} else {
		return routeParam
	}
}
