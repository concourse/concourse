package api

import (
	"net/http"
	"path/filepath"

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
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/pipelines"
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

	teamsDB teamserver.TeamsDB,
	workerDB workerserver.WorkerDB,
	containerDB containerserver.ContainerDB,
	volumesDB volumeserver.VolumesDB,
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

	cliDownloadsDir string,
	version string,
) (http.Handler, error) {
	absCLIDownloadsDir, err := filepath.Abs(cliDownloadsDir)
	if err != nil {
		return nil, err
	}

	pipelineHandlerFactory := pipelines.NewHandlerFactory(pipelineDBFactory, teamDBFactory)
	buildHandlerFactory := buildserver.NewScopedHandlerFactory(logger, teamDBFactory)

	authServer := authserver.NewServer(
		logger,
		externalURL,
		oAuthBaseURL,
		tokenGenerator,
		providerFactory,
		teamDBFactory,
	)

	buildServer := buildserver.NewServer(
		logger,
		externalURL,
		engine,
		workerClient,
		teamDBFactory,
		eventHandlerFactory,
		drain,
	)

	jobServer := jobserver.NewServer(logger, schedulerFactory, externalURL)
	resourceServer := resourceserver.NewServer(logger, scannerFactory)
	versionServer := versionserver.NewServer(logger, externalURL)
	pipeServer := pipes.NewServer(logger, peerURL, externalURL, pipeDB)

	pipelineServer := pipelineserver.NewServer(logger, teamDBFactory, pipelinesDB)

	configServer := configserver.NewServer(logger, teamDBFactory, configValidator)

	workerServer := workerserver.NewServer(logger, workerDB, teamDBFactory)

	logLevelServer := loglevelserver.NewServer(logger, sink)

	cliServer := cliserver.NewServer(logger, absCLIDownloadsDir)

	containerServer := containerserver.NewServer(logger, workerClient, containerDB, teamDBFactory)

	volumesServer := volumeserver.NewServer(logger, volumesDB, teamDBFactory)

	teamServer := teamserver.NewServer(logger, teamDBFactory, teamsDB)

	infoServer := infoserver.NewServer(logger, version)

	handlers := map[string]http.Handler{
		atc.ListAuthMethods: http.HandlerFunc(authServer.ListAuthMethods),
		atc.GetAuthToken:    http.HandlerFunc(authServer.GetAuthToken),

		atc.GetConfig:  http.HandlerFunc(configServer.GetConfig),
		atc.SaveConfig: http.HandlerFunc(configServer.SaveConfig),

		atc.GetBuild:            buildHandlerFactory.HandlerFor(buildServer.GetBuild, true),
		atc.ListBuilds:          http.HandlerFunc(buildServer.ListBuilds),
		atc.CreateBuild:         http.HandlerFunc(buildServer.CreateBuild),
		atc.BuildResources:      buildHandlerFactory.HandlerFor(buildServer.BuildResources, true),
		atc.AbortBuild:          http.HandlerFunc(buildServer.AbortBuild),
		atc.GetBuildPlan:        buildHandlerFactory.HandlerFor(buildServer.GetBuildPlan, true),
		atc.GetBuildPreparation: buildHandlerFactory.HandlerFor(buildServer.GetBuildPreparation, false),
		atc.BuildEvents:         buildHandlerFactory.HandlerFor(buildServer.BuildEvents, false),

		atc.ListJobs:       pipelineHandlerFactory.HandlerFor(jobServer.ListJobs, true),
		atc.GetJob:         pipelineHandlerFactory.HandlerFor(jobServer.GetJob, true),
		atc.ListJobBuilds:  pipelineHandlerFactory.HandlerFor(jobServer.ListJobBuilds, true),   // authorized or public
		atc.ListJobInputs:  pipelineHandlerFactory.HandlerFor(jobServer.ListJobInputs, false),  // authorized
		atc.GetJobBuild:    pipelineHandlerFactory.HandlerFor(jobServer.GetJobBuild, true),     // authorized or public
		atc.CreateJobBuild: pipelineHandlerFactory.HandlerFor(jobServer.CreateJobBuild, false), // authorized
		atc.PauseJob:       pipelineHandlerFactory.HandlerFor(jobServer.PauseJob, false),       // authorized
		atc.UnpauseJob:     pipelineHandlerFactory.HandlerFor(jobServer.UnpauseJob, false),     // authorized
		atc.JobBadge:       pipelineHandlerFactory.HandlerFor(jobServer.JobBadge, true),        // authorized or public

		atc.ListAllPipelines: http.HandlerFunc(pipelineServer.ListAllPipelines),
		atc.ListPipelines:    http.HandlerFunc(pipelineServer.ListPipelines),
		atc.GetPipeline:      http.HandlerFunc(pipelineServer.GetPipeline),
		atc.DeletePipeline:   pipelineHandlerFactory.HandlerFor(pipelineServer.DeletePipeline, false), // authorized
		atc.OrderPipelines:   http.HandlerFunc(pipelineServer.OrderPipelines),
		atc.PausePipeline:    pipelineHandlerFactory.HandlerFor(pipelineServer.PausePipeline, false),   // authorized
		atc.UnpausePipeline:  pipelineHandlerFactory.HandlerFor(pipelineServer.UnpausePipeline, false), // authorized
		atc.RevealPipeline:   pipelineHandlerFactory.HandlerFor(pipelineServer.RevealPipeline, false),  // authorized
		atc.ConcealPipeline:  pipelineHandlerFactory.HandlerFor(pipelineServer.ConcealPipeline, false), // authorized
		atc.GetVersionsDB:    pipelineHandlerFactory.HandlerFor(pipelineServer.GetVersionsDB, false),   // authorized
		atc.RenamePipeline:   pipelineHandlerFactory.HandlerFor(pipelineServer.RenamePipeline, false),  // authorized

		atc.ListResources:   pipelineHandlerFactory.HandlerFor(resourceServer.ListResources, true),    // authorized or public
		atc.GetResource:     pipelineHandlerFactory.HandlerFor(resourceServer.GetResource, true),      // authorized or public
		atc.PauseResource:   pipelineHandlerFactory.HandlerFor(resourceServer.PauseResource, false),   // authorized
		atc.UnpauseResource: pipelineHandlerFactory.HandlerFor(resourceServer.UnpauseResource, false), // authorized
		atc.CheckResource:   pipelineHandlerFactory.HandlerFor(resourceServer.CheckResource, false),   // authorized

		atc.ListResourceVersions:          pipelineHandlerFactory.HandlerFor(versionServer.ListResourceVersions, true),          // authorized or public
		atc.EnableResourceVersion:         pipelineHandlerFactory.HandlerFor(versionServer.EnableResourceVersion, false),        // authorized
		atc.DisableResourceVersion:        pipelineHandlerFactory.HandlerFor(versionServer.DisableResourceVersion, false),       // authorized
		atc.ListBuildsWithVersionAsInput:  pipelineHandlerFactory.HandlerFor(versionServer.ListBuildsWithVersionAsInput, true),  // authorized or public
		atc.ListBuildsWithVersionAsOutput: pipelineHandlerFactory.HandlerFor(versionServer.ListBuildsWithVersionAsOutput, true), // authorized or public

		atc.CreatePipe: http.HandlerFunc(pipeServer.CreatePipe),
		atc.WritePipe:  http.HandlerFunc(pipeServer.WritePipe),
		atc.ReadPipe:   http.HandlerFunc(pipeServer.ReadPipe),

		atc.ListWorkers:    http.HandlerFunc(workerServer.ListWorkers),
		atc.RegisterWorker: http.HandlerFunc(workerServer.RegisterWorker),

		atc.SetLogLevel: http.HandlerFunc(logLevelServer.SetMinLevel),
		atc.GetLogLevel: http.HandlerFunc(logLevelServer.GetMinLevel),

		atc.DownloadCLI: http.HandlerFunc(cliServer.Download),
		atc.GetInfo:     http.HandlerFunc(infoServer.Info),

		atc.ListContainers:  http.HandlerFunc(containerServer.ListContainers),
		atc.GetContainer:    http.HandlerFunc(containerServer.GetContainer),
		atc.HijackContainer: http.HandlerFunc(containerServer.HijackContainer),

		atc.ListVolumes: http.HandlerFunc(volumesServer.ListVolumes),

		atc.ListTeams: http.HandlerFunc(teamServer.ListTeams),
		atc.SetTeam:   http.HandlerFunc(teamServer.SetTeam),
	}

	return rata.NewRouter(atc.Routes, wrapper.Wrap(handlers))
}
