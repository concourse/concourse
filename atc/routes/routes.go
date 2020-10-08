package routes

import (
	"io"
	"net/http"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api"
	"github.com/concourse/concourse/atc/api/artifactserver"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/api/buildserver"
	"github.com/concourse/concourse/atc/api/ccserver"
	"github.com/concourse/concourse/atc/api/cliserver"
	"github.com/concourse/concourse/atc/api/configserver"
	"github.com/concourse/concourse/atc/api/containerserver"
	"github.com/concourse/concourse/atc/api/infoserver"
	"github.com/concourse/concourse/atc/api/jobserver"
	"github.com/concourse/concourse/atc/api/loglevelserver"
	"github.com/concourse/concourse/atc/api/pipelineserver"
	"github.com/concourse/concourse/atc/api/resourceserver"
	"github.com/concourse/concourse/atc/api/resourceserver/versionserver"
	"github.com/concourse/concourse/atc/api/teamserver"
	"github.com/concourse/concourse/atc/api/usersserver"
	"github.com/concourse/concourse/atc/api/volumeserver"
	"github.com/concourse/concourse/atc/api/wallserver"
	"github.com/concourse/concourse/atc/api/workerserver"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/gc"
	"github.com/concourse/concourse/atc/mainredirect"
	"github.com/concourse/concourse/atc/worker"
	"github.com/gorilla/mux"
)

func RouteNames() []string {
	names := []string{}
	Router().Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
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
	route := Router().Get(action)
	pairs := []string{}
	for key, val := range params {
		pairs = append(pairs, key, val)
	}
	url, err := route.URLPath(pairs...)
	if err != nil {
		return &http.Request{}, err
	}
	methods, err := route.GetMethods()
	if err != nil {
		return &http.Request{}, err
	}
	return http.NewRequest(methods[0], rae.host+url.String(), body)
}

type APIRouter struct {
	*mux.Router
}

func (r *APIRouter) CreatePathForRoute(action string, params map[string]string) (string, error) {
	pairs := []string{}
	for key, val := range params {
		pairs = append(pairs, key, val)
	}
	path, err := r.Get(action).URLPath(pairs...)
	if err != nil {
		return "", err
	}
	return path.String(), nil
}

type Wrappa interface {
	Wrap(map[string]http.Handler) map[string]http.Handler
}

func NewHandler(
	logger lager.Logger,

	externalURL string,
	clusterName string,

	wrapper Wrappa,

	dbTeamFactory db.TeamFactory,
	dbPipelineFactory db.PipelineFactory,
	dbJobFactory db.JobFactory,
	dbResourceFactory db.ResourceFactory,
	dbWorkerFactory db.WorkerFactory,
	volumeRepository db.VolumeRepository,
	containerRepository db.ContainerRepository,
	destroyer gc.Destroyer,
	dbBuildFactory db.BuildFactory,
	dbCheckFactory db.CheckFactory,
	dbResourceConfigFactory db.ResourceConfigFactory,
	dbUserFactory db.UserFactory,

	eventHandlerFactory buildserver.EventHandlerFactory,

	workerClient worker.Client,

	sink *lager.ReconfigurableSink,

	isTLSEnabled bool,

	cliDownloadsDir string,
	version string,
	workerVersion string,
	secretManager creds.Secrets,
	varSourcePool creds.VarSourcePool,
	credsManagers creds.Managers,
	interceptTimeoutFactory containerserver.InterceptTimeoutFactory,
	interceptUpdateInterval time.Duration,
	dbWall db.Wall,
	clock clock.Clock,
) (*APIRouter, error) {
	r := &APIRouter{mux.NewRouter()}

	absCLIDownloadsDir, err := filepath.Abs(cliDownloadsDir)
	if err != nil {
		return nil, err
	}

	pipelineHandlerFactory := pipelineserver.NewScopedHandlerFactory(dbTeamFactory)
	buildHandlerFactory := buildserver.NewScopedHandlerFactory(logger)
	teamHandlerFactory := api.NewTeamScopedHandlerFactory(logger, dbTeamFactory)

	buildServer := buildserver.NewServer(logger, externalURL, dbTeamFactory, dbBuildFactory, eventHandlerFactory, r)
	jobServer := jobserver.NewServer(logger, externalURL, secretManager, dbJobFactory, dbCheckFactory, r)
	resourceServer := resourceserver.NewServer(logger, secretManager, varSourcePool, dbCheckFactory, dbResourceFactory, dbResourceConfigFactory, r)

	versionServer := versionserver.NewServer(logger, externalURL, r)
	pipelineServer := pipelineserver.NewServer(logger, dbTeamFactory, dbPipelineFactory, externalURL, r)
	configServer := configserver.NewServer(logger, dbTeamFactory, secretManager)
	ccServer := ccserver.NewServer(logger, dbTeamFactory, externalURL)
	workerServer := workerserver.NewServer(logger, dbTeamFactory, dbWorkerFactory)
	logLevelServer := loglevelserver.NewServer(logger, sink)
	cliServer := cliserver.NewServer(logger, absCLIDownloadsDir)
	containerServer := containerserver.NewServer(logger, workerClient, secretManager, varSourcePool, interceptTimeoutFactory, interceptUpdateInterval, containerRepository, destroyer, clock)
	volumesServer := volumeserver.NewServer(logger, volumeRepository, destroyer)
	teamServer := teamserver.NewServer(logger, dbTeamFactory, externalURL, r)
	infoServer := infoserver.NewServer(logger, version, workerVersion, externalURL, clusterName, credsManagers)
	artifactServer := artifactserver.NewServer(logger, workerClient)
	usersServer := usersserver.NewServer(logger, dbUserFactory)
	wallServer := wallserver.NewServer(dbWall, logger)

	checkPipelineAccessHandlerFactory := auth.NewCheckPipelineAccessHandlerFactory(dbTeamFactory)

	r.Name(atc.CreateBuild).Path("/api/v1/teams/{team_name}/builds").Methods("POST")

	r.Name(atc.ListBuilds).Path("/api/v1/builds").Methods("GET")
	r.Name(atc.GetBuild).Path("/api/v1/builds/{build_id}").Methods("GET")
	r.Name(atc.GetBuildPlan).Path("/api/v1/builds/{build_id}/plan").Methods("GET")
	r.Name(atc.BuildEvents).Path("/api/v1/builds/{build_id}/events").Methods("GET")
	r.Name(atc.BuildResources).Path("/api/v1/builds/{build_id}/resources").Methods("GET")
	r.Name(atc.AbortBuild).Path("/api/v1/builds/{build_id}/abort").Methods("PUT")
	r.Name(atc.GetBuildPreparation).Path("/api/v1/builds/{build_id}/preparation").Methods("GET")
	r.Name(atc.ListBuildArtifacts).Path("/api/v1/builds/{build_id}/artifacts").Methods("GET")

	r.Name(atc.ListAllJobs).Path("/api/v1/jobs").Methods("GET")

	pr := r.PathPrefix("/api/v1/teams/{team_name}/pipelines/{pipeline_name}").
		Subrouter()

	pc := pr.PathPrefix("/config").Subrouter()
	pc.Use(func(handler http.Handler) http.Handler {
		return auth.CheckAuthorizationHandler(
			handler,
			auth.UnauthorizedRejector{},
		)
	})
	pc.Name(atc.SaveConfig).
		Methods("PUT").
		HandlerFunc(configServer.SaveConfig)
	pc.Name(atc.GetConfig).
		Methods("GET").
		HandlerFunc(configServer.GetConfig)

	jsr := pr.Path("/jobs").Subrouter()
	jsr.Use(func(handler http.Handler) http.Handler {
		return checkPipelineAccessHandlerFactory.HandlerFor(
			handler,
			auth.UnauthorizedRejector{},
		)
	})
	jsr.Name(atc.ListJobs).
		Methods("GET").
		HandlerFunc(pipelineHandlerFactory.HandlerFor(jobServer.ListJobs))

	jr := jsr.PathPrefix("/{job_name}").Subrouter()
	jr.Name(atc.GetJob).
		Methods("GET").
		HandlerFunc(pipelineHandlerFactory.HandlerFor(jobServer.GetJob))
	r.Name(atc.ListJobBuilds).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/builds").
		Methods("GET")
	r.Name(atc.CreateJobBuild).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/builds").
		Methods("POST")
	r.Name(atc.RerunJobBuild).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/builds/{build_name}").
		Methods("POST")
	r.Name(atc.ListJobInputs).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/inputs").
		Methods("GET")
	r.Name(atc.GetJobBuild).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/builds/{build_name}").
		Methods("GET")
	r.Name(atc.PauseJob).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/pause").
		Methods("PUT")
	r.Name(atc.UnpauseJob).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/unpause").
		Methods("PUT")
	r.Name(atc.ScheduleJob).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/schedule").
		Methods("PUT")
	r.Name(atc.JobBadge).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/badge").
		Methods("GET")
	r.Name(atc.MainJobBadge).Path("/api/v1/pipelines/{pipeline_name}/jobs/{job_name}/badge").Methods("GET")

	r.Name(atc.ClearTaskCache).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/jobs/{job_name}/tasks/{step_name}/cache").
		Methods("DELETE")

	r.Name(atc.ListAllPipelines).Path("/api/v1/pipelines").Methods("GET")
	r.Name(atc.ListPipelines).Path("/api/v1/teams/{team_name}/pipelines").Methods("GET")
	r.Name(atc.GetPipeline).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}").Methods("GET")
	r.Name(atc.DeletePipeline).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}").Methods("DELETE")
	r.Name(atc.OrderPipelines).Path("/api/v1/teams/{team_name}/pipelines/ordering").Methods("PUT")
	r.Name(atc.PausePipeline).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/pause").Methods("PUT")
	r.Name(atc.ArchivePipeline).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/archive").Methods("PUT")
	r.Name(atc.UnpausePipeline).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/unpause").Methods("PUT")
	r.Name(atc.ExposePipeline).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/expose").Methods("PUT")
	r.Name(atc.HidePipeline).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/hide").Methods("PUT")
	r.Name(atc.GetVersionsDB).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/versions-db").Methods("GET")
	r.Name(atc.RenamePipeline).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/rename").Methods("PUT")
	r.Name(atc.ListPipelineBuilds).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/builds").Methods("GET")
	r.Name(atc.CreatePipelineBuild).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/builds").Methods("POST")
	r.Name(atc.PipelineBadge).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/badge").Methods("GET")

	r.Name(atc.ListAllResources).Path("/api/v1/resources").Methods("GET")
	r.Name(atc.ListResources).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources").
		Methods("GET")
	r.Name(atc.ListResourceTypes).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resource-types").
		Methods("GET")
	r.Name(atc.GetResource).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}").
		Methods("GET")
	r.Name(atc.CheckResource).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/check").
		Methods("POST")
	r.Name(atc.CheckResourceWebHook).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/check/webhook").
		Methods("POST")
	r.Name(atc.CheckResourceType).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resource-types/{resource_type_name}/check").
		Methods("POST")

	r.Name(atc.ListResourceVersions).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/versions").
		Methods("GET")
	r.Name(atc.GetResourceVersion).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/versions/{resource_config_version_id}").
		Methods("GET")
	r.Name(atc.EnableResourceVersion).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/versions/{resource_config_version_id}/enable").
		Methods("PUT")
	r.Name(atc.DisableResourceVersion).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/versions/{resource_config_version_id}/disable").
		Methods("PUT")
	r.Name(atc.PinResourceVersion).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/versions/{resource_config_version_id}/pin").
		Methods("PUT")
	r.Name(atc.UnpinResource).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/unpin").Methods("PUT")
	r.Name(atc.SetPinCommentOnResource).Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/pin_comment").Methods("PUT")
	r.Name(atc.ListBuildsWithVersionAsInput).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/versions/{resource_config_version_id}/input_to").
		Methods("GET")
	r.Name(atc.ListBuildsWithVersionAsOutput).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/versions/{resource_config_version_id}/output_of").
		Methods("GET")
	r.Name(atc.GetResourceCausality).
		Path("/api/v1/teams/{team_name}/pipelines/{pipeline_name}/resources/{resource_name}/versions/{resource_version_id}/causality").
		Methods("GET")

	r.Name(atc.GetCC).Path("/api/v1/teams/{team_name}/cc.xml").Methods("GET")

	r.Name(atc.ListWorkers).Path("/api/v1/workers").Methods("GET")
	r.Name(atc.RegisterWorker).Path("/api/v1/workers").Methods("POST")
	r.Name(atc.LandWorker).Path("/api/v1/workers/{worker_name}/land").Methods("PUT")
	r.Name(atc.RetireWorker).Path("/api/v1/workers/{worker_name}/retire").Methods("PUT")
	r.Name(atc.PruneWorker).Path("/api/v1/workers/{worker_name}/prune").Methods("PUT")
	r.Name(atc.HeartbeatWorker).Path("/api/v1/workers/{worker_name}/heartbeat").Methods("PUT")
	r.Name(atc.DeleteWorker).Path("/api/v1/workers/{worker_name}").Methods("DELETE")

	r.Name(atc.GetLogLevel).Path("/api/v1/log-level").Methods("GET")
	r.Name(atc.SetLogLevel).Path("/api/v1/log-level").Methods("PUT")

	r.Name(atc.DownloadCLI).Path("/api/v1/cli").Methods("GET")
	r.Name(atc.GetInfo).Path("/api/v1/info").Methods("GET")
	r.Name(atc.GetInfoCreds).Path("/api/v1/info/creds").Methods("GET")

	r.Name(atc.GetUser).Path("/api/v1/user").Methods("GET")
	r.Name(atc.ListActiveUsersSince).Path("/api/v1/users").Methods("GET")

	r.Name(atc.ListDestroyingContainers).Path("/api/v1/containers/destroying").Methods("GET")
	r.Name(atc.ReportWorkerContainers).Path("/api/v1/containers/report").Methods("PUT")
	r.Name(atc.ListContainers).Path("/api/v1/teams/{team_name}/containers").Methods("GET")
	r.Name(atc.GetContainer).Path("/api/v1/teams/{team_name}/containers/{id}").Methods("GET")
	r.Name(atc.HijackContainer).Path("/api/v1/teams/{team_name}/containers/{id}/hijack").Methods("GET")

	r.Name(atc.ListVolumes).Path("/api/v1/teams/{team_name}/volumes").Methods("GET")
	r.Name(atc.ListDestroyingVolumes).Path("/api/v1/volumes/destroying").Methods("GET")
	r.Name(atc.ReportWorkerVolumes).Path("/api/v1/volumes/report").Methods("PUT")

	r.Name(atc.ListTeams).Path("/api/v1/teams").Methods("GET")
	r.Name(atc.GetTeam).Path("/api/v1/teams/{team_name}").Methods("GET")
	r.Name(atc.SetTeam).Path("/api/v1/teams/{team_name}").Methods("PUT")
	r.Name(atc.RenameTeam).Path("/api/v1/teams/{team_name}/rename").Methods("PUT")
	r.Name(atc.DestroyTeam).Path("/api/v1/teams/{team_name}").Methods("DELETE")
	r.Name(atc.ListTeamBuilds).Path("/api/v1/teams/{team_name}/builds").Methods("GET")

	r.Name(atc.CreateArtifact).Path("/api/v1/teams/{team_name}/artifacts").Methods("POST")
	r.Name(atc.GetArtifact).Path("/api/v1/teams/{team_name}/artifacts/{artifact_id}").Methods("GET")

	r.Name(atc.GetWall).Path("/api/v1/wall").Methods("GET")
	r.Name(atc.SetWall).Path("/api/v1/wall").Methods("PUT")
	r.Name(atc.ClearWall).Path("/api/v1/wall").Methods("DELETE")

	handlers := map[string]http.Handler{
		atc.GetCC: http.HandlerFunc(ccServer.GetCC),

		atc.ListBuilds:          http.HandlerFunc(buildServer.ListBuilds),
		atc.CreateBuild:         teamHandlerFactory.HandlerFor(buildServer.CreateBuild),
		atc.GetBuild:            buildHandlerFactory.HandlerFor(buildServer.GetBuild),
		atc.BuildResources:      buildHandlerFactory.HandlerFor(buildServer.BuildResources),
		atc.AbortBuild:          buildHandlerFactory.HandlerFor(buildServer.AbortBuild),
		atc.GetBuildPlan:        buildHandlerFactory.HandlerFor(buildServer.GetBuildPlan),
		atc.GetBuildPreparation: buildHandlerFactory.HandlerFor(buildServer.GetBuildPreparation),
		atc.BuildEvents:         buildHandlerFactory.HandlerFor(buildServer.BuildEvents),
		atc.ListBuildArtifacts:  buildHandlerFactory.HandlerFor(buildServer.GetBuildArtifacts),

		atc.ListAllJobs:    http.HandlerFunc(jobServer.ListAllJobs),
		atc.ListJobBuilds:  pipelineHandlerFactory.HandlerFor(jobServer.ListJobBuilds),
		atc.ListJobInputs:  pipelineHandlerFactory.HandlerFor(jobServer.ListJobInputs),
		atc.GetJobBuild:    pipelineHandlerFactory.HandlerFor(jobServer.GetJobBuild),
		atc.CreateJobBuild: pipelineHandlerFactory.HandlerFor(jobServer.CreateJobBuild),
		atc.RerunJobBuild:  pipelineHandlerFactory.HandlerFor(jobServer.RerunJobBuild),
		atc.PauseJob:       pipelineHandlerFactory.HandlerFor(jobServer.PauseJob),
		atc.UnpauseJob:     pipelineHandlerFactory.HandlerFor(jobServer.UnpauseJob),
		atc.ScheduleJob:    pipelineHandlerFactory.HandlerFor(jobServer.ScheduleJob),
		atc.JobBadge:       pipelineHandlerFactory.HandlerFor(jobServer.JobBadge),
		atc.MainJobBadge:   mainredirect.Handler{Route: atc.JobBadge, PathBuilder: r},

		atc.ClearTaskCache: pipelineHandlerFactory.HandlerFor(jobServer.ClearTaskCache),

		atc.ListAllPipelines:    http.HandlerFunc(pipelineServer.ListAllPipelines),
		atc.ListPipelines:       http.HandlerFunc(pipelineServer.ListPipelines),
		atc.GetPipeline:         pipelineHandlerFactory.HandlerFor(pipelineServer.GetPipeline),
		atc.DeletePipeline:      pipelineHandlerFactory.HandlerFor(pipelineServer.DeletePipeline),
		atc.OrderPipelines:      http.HandlerFunc(pipelineServer.OrderPipelines),
		atc.PausePipeline:       pipelineHandlerFactory.HandlerFor(pipelineServer.PausePipeline),
		atc.ArchivePipeline:     pipelineHandlerFactory.HandlerFor(pipelineServer.ArchivePipeline),
		atc.UnpausePipeline:     pipelineHandlerFactory.HandlerFor(pipelineServer.UnpausePipeline),
		atc.ExposePipeline:      pipelineHandlerFactory.HandlerFor(pipelineServer.ExposePipeline),
		atc.HidePipeline:        pipelineHandlerFactory.HandlerFor(pipelineServer.HidePipeline),
		atc.GetVersionsDB:       pipelineHandlerFactory.HandlerFor(pipelineServer.GetVersionsDB),
		atc.RenamePipeline:      pipelineHandlerFactory.HandlerFor(pipelineServer.RenamePipeline),
		atc.ListPipelineBuilds:  pipelineHandlerFactory.HandlerFor(pipelineServer.ListPipelineBuilds),
		atc.CreatePipelineBuild: pipelineHandlerFactory.HandlerFor(pipelineServer.CreateBuild),
		atc.PipelineBadge:       pipelineHandlerFactory.HandlerFor(pipelineServer.PipelineBadge),

		atc.ListAllResources:        http.HandlerFunc(resourceServer.ListAllResources),
		atc.ListResources:           pipelineHandlerFactory.HandlerFor(resourceServer.ListResources),
		atc.ListResourceTypes:       pipelineHandlerFactory.HandlerFor(resourceServer.ListVersionedResourceTypes),
		atc.GetResource:             pipelineHandlerFactory.HandlerFor(resourceServer.GetResource),
		atc.UnpinResource:           pipelineHandlerFactory.HandlerFor(resourceServer.UnpinResource),
		atc.SetPinCommentOnResource: pipelineHandlerFactory.HandlerFor(resourceServer.SetPinCommentOnResource),
		atc.CheckResource:           pipelineHandlerFactory.HandlerFor(resourceServer.CheckResource),
		atc.CheckResourceWebHook:    pipelineHandlerFactory.HandlerFor(resourceServer.CheckResourceWebHook),
		atc.CheckResourceType:       pipelineHandlerFactory.HandlerFor(resourceServer.CheckResourceType),

		atc.ListResourceVersions:          pipelineHandlerFactory.HandlerFor(versionServer.ListResourceVersions),
		atc.GetResourceVersion:            pipelineHandlerFactory.HandlerFor(versionServer.GetResourceVersion),
		atc.EnableResourceVersion:         pipelineHandlerFactory.HandlerFor(versionServer.EnableResourceVersion),
		atc.DisableResourceVersion:        pipelineHandlerFactory.HandlerFor(versionServer.DisableResourceVersion),
		atc.PinResourceVersion:            pipelineHandlerFactory.HandlerFor(versionServer.PinResourceVersion),
		atc.ListBuildsWithVersionAsInput:  pipelineHandlerFactory.HandlerFor(versionServer.ListBuildsWithVersionAsInput),
		atc.ListBuildsWithVersionAsOutput: pipelineHandlerFactory.HandlerFor(versionServer.ListBuildsWithVersionAsOutput),
		atc.GetResourceCausality:          pipelineHandlerFactory.HandlerFor(versionServer.GetCausality),

		atc.ListWorkers:     http.HandlerFunc(workerServer.ListWorkers),
		atc.RegisterWorker:  http.HandlerFunc(workerServer.RegisterWorker),
		atc.LandWorker:      http.HandlerFunc(workerServer.LandWorker),
		atc.RetireWorker:    http.HandlerFunc(workerServer.RetireWorker),
		atc.PruneWorker:     http.HandlerFunc(workerServer.PruneWorker),
		atc.HeartbeatWorker: http.HandlerFunc(workerServer.HeartbeatWorker),
		atc.DeleteWorker:    http.HandlerFunc(workerServer.DeleteWorker),

		atc.SetLogLevel: http.HandlerFunc(logLevelServer.SetMinLevel),
		atc.GetLogLevel: http.HandlerFunc(logLevelServer.GetMinLevel),

		atc.DownloadCLI:  http.HandlerFunc(cliServer.Download),
		atc.GetInfo:      http.HandlerFunc(infoServer.Info),
		atc.GetInfoCreds: http.HandlerFunc(infoServer.Creds),

		atc.GetUser:              http.HandlerFunc(usersServer.GetUser),
		atc.ListActiveUsersSince: http.HandlerFunc(usersServer.GetUsersSince),

		atc.ListContainers:           teamHandlerFactory.HandlerFor(containerServer.ListContainers),
		atc.GetContainer:             teamHandlerFactory.HandlerFor(containerServer.GetContainer),
		atc.HijackContainer:          teamHandlerFactory.HandlerFor(containerServer.HijackContainer),
		atc.ListDestroyingContainers: http.HandlerFunc(containerServer.ListDestroyingContainers),
		atc.ReportWorkerContainers:   http.HandlerFunc(containerServer.ReportWorkerContainers),

		atc.ListVolumes:           teamHandlerFactory.HandlerFor(volumesServer.ListVolumes),
		atc.ListDestroyingVolumes: http.HandlerFunc(volumesServer.ListDestroyingVolumes),
		atc.ReportWorkerVolumes:   http.HandlerFunc(volumesServer.ReportWorkerVolumes),

		atc.ListTeams:      http.HandlerFunc(teamServer.ListTeams),
		atc.GetTeam:        http.HandlerFunc(teamServer.GetTeam),
		atc.SetTeam:        http.HandlerFunc(teamServer.SetTeam),
		atc.RenameTeam:     http.HandlerFunc(teamServer.RenameTeam),
		atc.DestroyTeam:    http.HandlerFunc(teamServer.DestroyTeam),
		atc.ListTeamBuilds: http.HandlerFunc(teamServer.ListTeamBuilds),

		atc.CreateArtifact: teamHandlerFactory.HandlerFor(artifactServer.CreateArtifact),
		atc.GetArtifact:    teamHandlerFactory.HandlerFor(artifactServer.GetArtifact),

		atc.GetWall:   http.HandlerFunc(wallServer.GetWall),
		atc.SetWall:   http.HandlerFunc(wallServer.SetWall),
		atc.ClearWall: http.HandlerFunc(wallServer.ClearWall),
	}

	for action, handler := range wrapper.Wrap(handlers) {
		r.Get(action).Handler(handler)
	}
	return r, nil
}

func Router() *APIRouter {
	router, _ := NewHandler(
		lagertest.NewTestLogger(""),

		"",
		"",

		&identityWrappa{},

		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,

		nil,

		nil,

		nil,

		false,

		"",
		"",
		"",
		nil,
		nil,
		nil,
		nil,
		0,
		nil,
		nil,
	)
	return router
}

type identityWrappa struct{}

func (_ *identityWrappa) Wrap(handlers map[string]http.Handler) map[string]http.Handler {
	return handlers
}
