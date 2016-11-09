package api

import (
	"net/http"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/authserver"
	"github.com/concourse/atc/api/buildserver"
	"github.com/concourse/atc/api/cliserver"
	"github.com/concourse/atc/api/configserver"
	"github.com/concourse/atc/api/containerserver"
	"github.com/concourse/atc/api/infoserver"
	"github.com/concourse/atc/api/jobserver"
	"github.com/concourse/atc/api/loglevelserver"
	"github.com/concourse/atc/api/pipelineserver"
	"github.com/concourse/atc/api/pipes"
	"github.com/concourse/atc/api/resourceserver"
	"github.com/concourse/atc/api/resourceserver/versionserver"
	"github.com/concourse/atc/api/teamserver"
	"github.com/concourse/atc/api/volumeserver"
	"github.com/concourse/atc/api/workerserver"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/mainredirect"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/wrappa"
)

func NewHandler(
	logger lager.Logger,

	externalURL string,

	wrapper wrappa.Wrappa,

	tokenGenerator auth.TokenGenerator,
	providerFactory auth.ProviderFactory,
	oAuthBaseURL string,

	pipelineDBFactory db.PipelineDBFactory,
	teamDBFactory db.TeamDBFactory,
	dbTeamFactory dbng.TeamFactory,
	dbWorkerFactory dbng.WorkerFactory,
	volumeFactory dbng.VolumeFactory,

	teamsDB teamserver.TeamsDB,
	workerDB workerserver.WorkerDB,
	buildsDB buildserver.BuildsDB,
	containerDB containerserver.ContainerDB,
	pipeDB pipes.PipeDB,
	pipelinesDB db.PipelinesDB,

	configValidator configserver.ConfigValidator,
	peerURL string,
	eventHandlerFactory buildserver.EventHandlerFactory,
	drain <-chan struct{},

	engine engine.Engine,
	workerClient worker.Client,

	schedulerFactory jobserver.SchedulerFactory,
	scannerFactory resourceserver.ScannerFactory,

	sink *lager.ReconfigurableSink,

	expire time.Duration,

	cliDownloadsDir string,
	version string,
) (http.Handler, error) {
	absCLIDownloadsDir, err := filepath.Abs(cliDownloadsDir)
	if err != nil {
		return nil, err
	}

	pipelineHandlerFactory := pipelineserver.NewScopedHandlerFactory(pipelineDBFactory, teamDBFactory)
	buildHandlerFactory := buildserver.NewScopedHandlerFactory(logger)
	teamHandlerFactory := NewTeamScopedHandlerFactory(logger, teamDBFactory)

	authServer := authserver.NewServer(
		logger,
		externalURL,
		oAuthBaseURL,
		tokenGenerator,
		providerFactory,
		teamDBFactory,
		expire,
	)

	buildServer := buildserver.NewServer(
		logger,
		externalURL,
		engine,
		workerClient,
		teamDBFactory,
		buildsDB,
		eventHandlerFactory,
		drain,
	)

	jobServer := jobserver.NewServer(logger, schedulerFactory, externalURL)
	resourceServer := resourceserver.NewServer(logger, scannerFactory)
	versionServer := versionserver.NewServer(logger, externalURL)
	pipeServer := pipes.NewServer(logger, peerURL, externalURL, pipeDB)

	pipelineServer := pipelineserver.NewServer(logger, teamDBFactory, pipelinesDB)

	configServer := configserver.NewServer(logger, teamDBFactory, configValidator)

	workerServer := workerserver.NewServer(logger, workerDB, teamDBFactory, dbTeamFactory, dbWorkerFactory)

	logLevelServer := loglevelserver.NewServer(logger, sink)

	cliServer := cliserver.NewServer(logger, absCLIDownloadsDir)

	containerServer := containerserver.NewServer(logger, workerClient, containerDB, teamDBFactory)

	volumesServer := volumeserver.NewServer(logger, volumeFactory)

	teamServer := teamserver.NewServer(logger, teamDBFactory, teamsDB)

	infoServer := infoserver.NewServer(logger, version)

	handlers := map[string]http.Handler{
		atc.ListAuthMethods: http.HandlerFunc(authServer.ListAuthMethods),
		atc.GetAuthToken:    http.HandlerFunc(authServer.GetAuthToken),

		atc.GetConfig:  http.HandlerFunc(configServer.GetConfig),
		atc.SaveConfig: http.HandlerFunc(configServer.SaveConfig),

		atc.GetBuild:            buildHandlerFactory.HandlerFor(buildServer.GetBuild),
		atc.ListBuilds:          http.HandlerFunc(buildServer.ListBuilds),
		atc.CreateBuild:         teamHandlerFactory.HandlerFor(buildServer.CreateBuild),
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

		atc.ListAllPipelines: http.HandlerFunc(pipelineServer.ListAllPipelines),
		atc.ListPipelines:    http.HandlerFunc(pipelineServer.ListPipelines),
		atc.GetPipeline:      pipelineHandlerFactory.HandlerFor(pipelineServer.GetPipeline),
		atc.DeletePipeline:   pipelineHandlerFactory.HandlerFor(pipelineServer.DeletePipeline),
		atc.OrderPipelines:   http.HandlerFunc(pipelineServer.OrderPipelines),
		atc.PausePipeline:    pipelineHandlerFactory.HandlerFor(pipelineServer.PausePipeline),
		atc.UnpausePipeline:  pipelineHandlerFactory.HandlerFor(pipelineServer.UnpausePipeline),
		atc.ExposePipeline:   pipelineHandlerFactory.HandlerFor(pipelineServer.ExposePipeline),
		atc.HidePipeline:     pipelineHandlerFactory.HandlerFor(pipelineServer.HidePipeline),
		atc.GetVersionsDB:    pipelineHandlerFactory.HandlerFor(pipelineServer.GetVersionsDB),
		atc.RenamePipeline:   pipelineHandlerFactory.HandlerFor(pipelineServer.RenamePipeline),

		atc.ListResources:   pipelineHandlerFactory.HandlerFor(resourceServer.ListResources),
		atc.GetResource:     pipelineHandlerFactory.HandlerFor(resourceServer.GetResource),
		atc.PauseResource:   pipelineHandlerFactory.HandlerFor(resourceServer.PauseResource),
		atc.UnpauseResource: pipelineHandlerFactory.HandlerFor(resourceServer.UnpauseResource),
		atc.CheckResource:   pipelineHandlerFactory.HandlerFor(resourceServer.CheckResource),

		atc.ListResourceVersions:          pipelineHandlerFactory.HandlerFor(versionServer.ListResourceVersions),
		atc.EnableResourceVersion:         pipelineHandlerFactory.HandlerFor(versionServer.EnableResourceVersion),
		atc.DisableResourceVersion:        pipelineHandlerFactory.HandlerFor(versionServer.DisableResourceVersion),
		atc.ListBuildsWithVersionAsInput:  pipelineHandlerFactory.HandlerFor(versionServer.ListBuildsWithVersionAsInput),
		atc.ListBuildsWithVersionAsOutput: pipelineHandlerFactory.HandlerFor(versionServer.ListBuildsWithVersionAsOutput),

		atc.CreatePipe: http.HandlerFunc(pipeServer.CreatePipe),
		atc.WritePipe:  http.HandlerFunc(pipeServer.WritePipe),
		atc.ReadPipe:   http.HandlerFunc(pipeServer.ReadPipe),

		atc.ListWorkers:    teamHandlerFactory.HandlerFor(workerServer.ListWorkers),
		atc.RegisterWorker: http.HandlerFunc(workerServer.RegisterWorker),
		atc.LandWorker:     http.HandlerFunc(workerServer.LandWorker),

		atc.SetLogLevel: http.HandlerFunc(logLevelServer.SetMinLevel),
		atc.GetLogLevel: http.HandlerFunc(logLevelServer.GetMinLevel),

		atc.DownloadCLI: http.HandlerFunc(cliServer.Download),
		atc.GetInfo:     http.HandlerFunc(infoServer.Info),
		atc.GetUser:     http.HandlerFunc(authServer.GetUser),

		atc.ListContainers:  teamHandlerFactory.HandlerFor(containerServer.ListContainers),
		atc.GetContainer:    teamHandlerFactory.HandlerFor(containerServer.GetContainer),
		atc.HijackContainer: teamHandlerFactory.HandlerFor(containerServer.HijackContainer),

		atc.ListVolumes: teamHandlerFactory.HandlerFor(volumesServer.ListVolumes),

		atc.ListTeams:   http.HandlerFunc(teamServer.ListTeams),
		atc.SetTeam:     http.HandlerFunc(teamServer.SetTeam),
		atc.DestroyTeam: http.HandlerFunc(teamServer.DestroyTeam),
	}

	return rata.NewRouter(atc.Routes, wrapper.Wrap(handlers))
}
