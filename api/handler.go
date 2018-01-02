package api

import (
	"net/http"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/buildserver"
	"github.com/concourse/atc/api/cliserver"
	"github.com/concourse/atc/api/configserver"
	"github.com/concourse/atc/api/containerserver"
	"github.com/concourse/atc/api/infoserver"
	"github.com/concourse/atc/api/jobserver"
	"github.com/concourse/atc/api/legacyserver"
	"github.com/concourse/atc/api/loglevelserver"
	"github.com/concourse/atc/api/pipelineserver"
	"github.com/concourse/atc/api/pipes"
	"github.com/concourse/atc/api/resourceserver"
	"github.com/concourse/atc/api/resourceserver/versionserver"
	"github.com/concourse/atc/api/teamserver"
	"github.com/concourse/atc/api/volumeserver"
	"github.com/concourse/atc/api/workerserver"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/mainredirect"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/wrappa"
)

func NewHandler(
	logger lager.Logger,

	externalURL string,

	wrapper wrappa.Wrappa,

	oAuthBaseURL string,

	dbTeamFactory db.TeamFactory,
	dbPipelineFactory db.PipelineFactory,
	dbWorkerFactory db.WorkerFactory,
	volumeFactory db.VolumeFactory,
	containerRepository db.ContainerRepository,
	dbBuildFactory db.BuildFactory,

	peerURL string,
	eventHandlerFactory buildserver.EventHandlerFactory,
	drain <-chan struct{},

	engine engine.Engine,
	workerClient worker.Client,
	workerProvider worker.WorkerProvider,

	schedulerFactory jobserver.SchedulerFactory,
	scannerFactory resourceserver.ScannerFactory,

	sink *lager.ReconfigurableSink,

	expire time.Duration,

	isTLSEnabled bool,

	cliDownloadsDir string,
	version string,
	workerVersion string,
	variablesFactory creds.VariablesFactory,
	interceptTimeoutFactory containerserver.InterceptTimeoutFactory,
) (http.Handler, error) {

	absCLIDownloadsDir, err := filepath.Abs(cliDownloadsDir)
	if err != nil {
		return nil, err
	}

	pipelineHandlerFactory := pipelineserver.NewScopedHandlerFactory(dbTeamFactory)
	buildHandlerFactory := buildserver.NewScopedHandlerFactory(logger)
	teamHandlerFactory := NewTeamScopedHandlerFactory(logger, dbTeamFactory)

	buildServer := buildserver.NewServer(logger, externalURL, engine, workerClient, dbTeamFactory, dbBuildFactory, eventHandlerFactory, drain)
	jobServer := jobserver.NewServer(logger, schedulerFactory, externalURL, variablesFactory)
	resourceServer := resourceserver.NewServer(logger, scannerFactory)
	versionServer := versionserver.NewServer(logger, externalURL)
	pipeServer := pipes.NewServer(logger, peerURL, externalURL, dbTeamFactory)
	pipelineServer := pipelineserver.NewServer(logger, dbTeamFactory, dbPipelineFactory, engine)
	configServer := configserver.NewServer(logger, dbTeamFactory)
	workerServer := workerserver.NewServer(logger, dbTeamFactory, dbWorkerFactory, workerProvider)
	logLevelServer := loglevelserver.NewServer(logger, sink)
	cliServer := cliserver.NewServer(logger, absCLIDownloadsDir)
	containerServer := containerserver.NewServer(logger, workerClient, variablesFactory, interceptTimeoutFactory)
	volumesServer := volumeserver.NewServer(logger, volumeFactory)
	teamServer := teamserver.NewServer(logger, dbTeamFactory)
	infoServer := infoserver.NewServer(logger, version, workerVersion)
	legacyServer := legacyserver.NewServer(logger)

	handlers := map[string]http.Handler{
		atc.GetConfig:  http.HandlerFunc(configServer.GetConfig),
		atc.SaveConfig: http.HandlerFunc(configServer.SaveConfig),

		atc.ListBuilds:          http.HandlerFunc(buildServer.ListBuilds),
		atc.CreateBuild:         teamHandlerFactory.HandlerFor(buildServer.CreateBuild),
		atc.GetBuild:            buildHandlerFactory.HandlerFor(buildServer.GetBuild),
		atc.BuildResources:      buildHandlerFactory.HandlerFor(buildServer.BuildResources),
		atc.AbortBuild:          buildHandlerFactory.HandlerFor(buildServer.AbortBuild),
		atc.GetBuildPlan:        buildHandlerFactory.HandlerFor(buildServer.GetBuildPlan),
		atc.GetBuildPreparation: buildHandlerFactory.HandlerFor(buildServer.GetBuildPreparation),
		atc.BuildEvents:         buildHandlerFactory.HandlerFor(buildServer.BuildEvents),

		atc.ListJobs:       pipelineHandlerFactory.HandlerFor(jobServer.ListJobs),
		atc.GetJob:         pipelineHandlerFactory.HandlerFor(jobServer.GetJob),
		atc.ListJobBuilds:  pipelineHandlerFactory.HandlerFor(jobServer.ListJobBuilds),
		atc.ListJobInputs:  pipelineHandlerFactory.HandlerFor(jobServer.ListJobInputs),
		atc.GetJobBuild:    pipelineHandlerFactory.HandlerFor(jobServer.GetJobBuild),
		atc.CreateJobBuild: pipelineHandlerFactory.HandlerFor(jobServer.CreateJobBuild),
		atc.PauseJob:       pipelineHandlerFactory.HandlerFor(jobServer.PauseJob),
		atc.UnpauseJob:     pipelineHandlerFactory.HandlerFor(jobServer.UnpauseJob),
		atc.JobBadge:       pipelineHandlerFactory.HandlerFor(jobServer.JobBadge),
		atc.MainJobBadge:   mainredirect.Handler{atc.Routes, atc.JobBadge},

		atc.ListAllPipelines:    http.HandlerFunc(pipelineServer.ListAllPipelines),
		atc.ListPipelines:       http.HandlerFunc(pipelineServer.ListPipelines),
		atc.GetPipeline:         pipelineHandlerFactory.HandlerFor(pipelineServer.GetPipeline),
		atc.DeletePipeline:      pipelineHandlerFactory.HandlerFor(pipelineServer.DeletePipeline),
		atc.OrderPipelines:      http.HandlerFunc(pipelineServer.OrderPipelines),
		atc.PausePipeline:       pipelineHandlerFactory.HandlerFor(pipelineServer.PausePipeline),
		atc.UnpausePipeline:     pipelineHandlerFactory.HandlerFor(pipelineServer.UnpausePipeline),
		atc.ExposePipeline:      pipelineHandlerFactory.HandlerFor(pipelineServer.ExposePipeline),
		atc.HidePipeline:        pipelineHandlerFactory.HandlerFor(pipelineServer.HidePipeline),
		atc.GetVersionsDB:       pipelineHandlerFactory.HandlerFor(pipelineServer.GetVersionsDB),
		atc.RenamePipeline:      pipelineHandlerFactory.HandlerFor(pipelineServer.RenamePipeline),
		atc.CreatePipelineBuild: pipelineHandlerFactory.HandlerFor(pipelineServer.CreateBuild),
		atc.PipelineBadge:       pipelineHandlerFactory.HandlerFor(pipelineServer.PipelineBadge),

		atc.ListResources:        pipelineHandlerFactory.HandlerFor(resourceServer.ListResources),
		atc.GetResource:          pipelineHandlerFactory.HandlerFor(resourceServer.GetResource),
		atc.PauseResource:        pipelineHandlerFactory.HandlerFor(resourceServer.PauseResource),
		atc.UnpauseResource:      pipelineHandlerFactory.HandlerFor(resourceServer.UnpauseResource),
		atc.CheckResource:        pipelineHandlerFactory.HandlerFor(resourceServer.CheckResource),
		atc.CheckResourceWebHook: pipelineHandlerFactory.HandlerFor(resourceServer.CheckResourceWebHook),

		atc.ListResourceVersions:          pipelineHandlerFactory.HandlerFor(versionServer.ListResourceVersions),
		atc.GetResourceVersion:            pipelineHandlerFactory.HandlerFor(versionServer.GetResourceVersion),
		atc.EnableResourceVersion:         pipelineHandlerFactory.HandlerFor(versionServer.EnableResourceVersion),
		atc.DisableResourceVersion:        pipelineHandlerFactory.HandlerFor(versionServer.DisableResourceVersion),
		atc.ListBuildsWithVersionAsInput:  pipelineHandlerFactory.HandlerFor(versionServer.ListBuildsWithVersionAsInput),
		atc.ListBuildsWithVersionAsOutput: pipelineHandlerFactory.HandlerFor(versionServer.ListBuildsWithVersionAsOutput),
		atc.GetResourceCausality:          pipelineHandlerFactory.HandlerFor(versionServer.GetCausality),

		atc.CreatePipe: http.HandlerFunc(pipeServer.CreatePipe),
		atc.WritePipe:  http.HandlerFunc(pipeServer.WritePipe),
		atc.ReadPipe:   http.HandlerFunc(pipeServer.ReadPipe),

		atc.ListWorkers:     teamHandlerFactory.HandlerFor(workerServer.ListWorkers),
		atc.RegisterWorker:  http.HandlerFunc(workerServer.RegisterWorker),
		atc.LandWorker:      http.HandlerFunc(workerServer.LandWorker),
		atc.RetireWorker:    http.HandlerFunc(workerServer.RetireWorker),
		atc.PruneWorker:     http.HandlerFunc(workerServer.PruneWorker),
		atc.HeartbeatWorker: http.HandlerFunc(workerServer.HeartbeatWorker),
		atc.DeleteWorker:    http.HandlerFunc(workerServer.DeleteWorker),

		atc.SetLogLevel: http.HandlerFunc(logLevelServer.SetMinLevel),
		atc.GetLogLevel: http.HandlerFunc(logLevelServer.GetMinLevel),

		atc.DownloadCLI: http.HandlerFunc(cliServer.Download),
		atc.GetInfo:     http.HandlerFunc(infoServer.Info),

		atc.ListContainers:  teamHandlerFactory.HandlerFor(containerServer.ListContainers),
		atc.GetContainer:    teamHandlerFactory.HandlerFor(containerServer.GetContainer),
		atc.HijackContainer: teamHandlerFactory.HandlerFor(containerServer.HijackContainer),

		atc.ListVolumes: teamHandlerFactory.HandlerFor(volumesServer.ListVolumes),

		atc.LegacyListAuthMethods: http.HandlerFunc(legacyServer.ListAuthMethods),
		atc.LegacyGetAuthToken:    http.HandlerFunc(legacyServer.GetAuthToken),
		atc.LegacyGetUser:         http.HandlerFunc(legacyServer.GetUser),

		atc.ListTeams:   http.HandlerFunc(teamServer.ListTeams),
		atc.SetTeam:     http.HandlerFunc(teamServer.SetTeam),
		atc.RenameTeam:  http.HandlerFunc(teamServer.RenameTeam),
		atc.DestroyTeam: http.HandlerFunc(teamServer.DestroyTeam),
	}

	return rata.NewRouter(atc.Routes, wrapper.Wrap(handlers))
}
