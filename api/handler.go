package api

import (
	"net/http"
	"path/filepath"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/authserver"
	"github.com/concourse/atc/api/buildserver"
	"github.com/concourse/atc/api/cliserver"
	"github.com/concourse/atc/api/configserver"
	"github.com/concourse/atc/api/containerserver"
	"github.com/concourse/atc/api/jobserver"
	"github.com/concourse/atc/api/loglevelserver"
	"github.com/concourse/atc/api/pipelineserver"
	"github.com/concourse/atc/api/pipes"
	"github.com/concourse/atc/api/resourceserver"
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
	providers auth.Providers,
	basicAuthEnabled bool,

	pipelineDBFactory db.PipelineDBFactory,
	configDB db.ConfigDB,

	buildsDB buildserver.BuildsDB,
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

	sink *lager.ReconfigurableSink,

	cliDownloadsDir string,
) (http.Handler, error) {
	absCLIDownloadsDir, err := filepath.Abs(cliDownloadsDir)
	if err != nil {
		return nil, err
	}

	pipelineHandlerFactory := pipelines.NewHandlerFactory(pipelineDBFactory)

	authServer := authserver.NewServer(
		logger,
		externalURL,
		tokenGenerator,
		providers,
		basicAuthEnabled,
	)

	buildServer := buildserver.NewServer(
		logger,
		engine,
		workerClient,
		buildsDB,
		configDB,
		eventHandlerFactory,
		drain,
	)

	jobServer := jobserver.NewServer(logger)
	resourceServer := resourceserver.NewServer(logger)
	pipeServer := pipes.NewServer(logger, peerURL, pipeDB)

	pipelineServer := pipelineserver.NewServer(logger, pipelinesDB)

	configServer := configserver.NewServer(logger, configDB, configValidator)

	workerServer := workerserver.NewServer(logger, workerDB)

	logLevelServer := loglevelserver.NewServer(logger, sink)

	cliServer := cliserver.NewServer(logger, absCLIDownloadsDir)

	containerServer := containerserver.NewServer(logger, workerClient, containerDB)

	volumesServer := volumeserver.NewServer(logger, volumesDB)

	handlers := map[string]http.Handler{
		atc.ListAuthMethods: http.HandlerFunc(authServer.ListAuthMethods),
		atc.GetAuthToken:    http.HandlerFunc(authServer.GetAuthToken),

		atc.GetConfig:  http.HandlerFunc(configServer.GetConfig),
		atc.SaveConfig: http.HandlerFunc(configServer.SaveConfig),

		atc.GetBuild:    http.HandlerFunc(buildServer.GetBuild),
		atc.ListBuilds:  http.HandlerFunc(buildServer.ListBuilds),
		atc.CreateBuild: http.HandlerFunc(buildServer.CreateBuild),
		atc.BuildEvents: http.HandlerFunc(buildServer.BuildEvents),
		atc.AbortBuild:  http.HandlerFunc(buildServer.AbortBuild),

		atc.ListJobs:      pipelineHandlerFactory.HandlerFor(jobServer.ListJobs),
		atc.GetJob:        pipelineHandlerFactory.HandlerFor(jobServer.GetJob),
		atc.ListJobBuilds: pipelineHandlerFactory.HandlerFor(jobServer.ListJobBuilds),
		atc.ListJobInputs: pipelineHandlerFactory.HandlerFor(jobServer.ListJobInputs),
		atc.GetJobBuild:   pipelineHandlerFactory.HandlerFor(jobServer.GetJobBuild),
		atc.PauseJob:      pipelineHandlerFactory.HandlerFor(jobServer.PauseJob),
		atc.UnpauseJob:    pipelineHandlerFactory.HandlerFor(jobServer.UnpauseJob),

		atc.ListPipelines:   http.HandlerFunc(pipelineServer.ListPipelines),
		atc.DeletePipeline:  pipelineHandlerFactory.HandlerFor(pipelineServer.DeletePipeline),
		atc.OrderPipelines:  http.HandlerFunc(pipelineServer.OrderPipelines),
		atc.PausePipeline:   pipelineHandlerFactory.HandlerFor(pipelineServer.PausePipeline),
		atc.UnpausePipeline: pipelineHandlerFactory.HandlerFor(pipelineServer.UnpausePipeline),

		atc.ListResources:          pipelineHandlerFactory.HandlerFor(resourceServer.ListResources),
		atc.EnableResourceVersion:  pipelineHandlerFactory.HandlerFor(resourceServer.EnableResourceVersion),
		atc.DisableResourceVersion: pipelineHandlerFactory.HandlerFor(resourceServer.DisableResourceVersion),
		atc.PauseResource:          pipelineHandlerFactory.HandlerFor(resourceServer.PauseResource),
		atc.UnpauseResource:        pipelineHandlerFactory.HandlerFor(resourceServer.UnpauseResource),

		atc.CreatePipe: http.HandlerFunc(pipeServer.CreatePipe),
		atc.WritePipe:  http.HandlerFunc(pipeServer.WritePipe),
		atc.ReadPipe:   http.HandlerFunc(pipeServer.ReadPipe),

		atc.ListWorkers:    http.HandlerFunc(workerServer.ListWorkers),
		atc.RegisterWorker: http.HandlerFunc(workerServer.RegisterWorker),

		atc.SetLogLevel: http.HandlerFunc(logLevelServer.SetMinLevel),
		atc.GetLogLevel: http.HandlerFunc(logLevelServer.GetMinLevel),

		atc.DownloadCLI: http.HandlerFunc(cliServer.Download),

		atc.ListContainers:  http.HandlerFunc(containerServer.ListContainers),
		atc.GetContainer:    http.HandlerFunc(containerServer.GetContainer),
		atc.HijackContainer: http.HandlerFunc(containerServer.HijackContainer),

		atc.ListVolumes: http.HandlerFunc(volumesServer.ListVolumes),
	}

	return rata.NewRouter(atc.Routes, wrapper.Wrap(handlers))
}
