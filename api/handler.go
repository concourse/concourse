package api

import (
	"net/http"
	"path/filepath"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/buildserver"
	"github.com/concourse/atc/api/cliserver"
	"github.com/concourse/atc/api/configserver"
	"github.com/concourse/atc/api/hijackserver"
	"github.com/concourse/atc/api/jobserver"
	"github.com/concourse/atc/api/loglevelserver"
	"github.com/concourse/atc/api/pipelineserver"
	"github.com/concourse/atc/api/pipes"
	"github.com/concourse/atc/api/resourceserver"
	"github.com/concourse/atc/api/workerserver"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/pipelines"
	"github.com/concourse/atc/worker"
)

func NewHandler(
	logger lager.Logger,
	validator auth.Validator,
	pipelineDBFactory db.PipelineDBFactory,

	configDB db.ConfigDB,

	buildsDB buildserver.BuildsDB,
	workerDB workerserver.WorkerDB,
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

	buildServer := buildserver.NewServer(
		logger,
		engine,
		workerClient,
		buildsDB,
		configDB,
		eventHandlerFactory,
		drain,
		validator,
	)

	hijackServer := hijackserver.NewServer(
		logger,
		workerClient,
	)

	jobServer := jobserver.NewServer(logger)
	resourceServer := resourceserver.NewServer(logger, validator)
	pipeServer := pipes.NewServer(logger, peerURL, pipeDB)

	pipelineServer := pipelineserver.NewServer(logger, pipelinesDB)

	configServer := configserver.NewServer(logger, configDB, configValidator)

	workerServer := workerserver.NewServer(logger, workerDB)

	logLevelServer := loglevelserver.NewServer(logger, sink)

	cliServer := cliserver.NewServer(logger, absCLIDownloadsDir)

	validate := func(handler http.Handler) http.Handler {
		return auth.Handler{
			Handler:   handler,
			Validator: validator,
		}
	}

	handlers := map[string]http.Handler{
		atc.GetConfig:  validate(http.HandlerFunc(configServer.GetConfig)),
		atc.SaveConfig: validate(http.HandlerFunc(configServer.SaveConfig)),

		atc.Hijack: validate(http.HandlerFunc(hijackServer.Hijack)),

		atc.ListBuilds:  http.HandlerFunc(buildServer.ListBuilds),
		atc.CreateBuild: validate(http.HandlerFunc(buildServer.CreateBuild)),
		atc.BuildEvents: http.HandlerFunc(buildServer.BuildEvents),
		atc.AbortBuild:  validate(http.HandlerFunc(buildServer.AbortBuild)),

		atc.ListJobs:      pipelineHandlerFactory.HandlerFor(jobServer.ListJobs),
		atc.GetJob:        pipelineHandlerFactory.HandlerFor(jobServer.GetJob),
		atc.ListJobBuilds: pipelineHandlerFactory.HandlerFor(jobServer.ListJobBuilds),
		atc.GetJobBuild:   pipelineHandlerFactory.HandlerFor(jobServer.GetJobBuild),
		atc.PauseJob:      validate(pipelineHandlerFactory.HandlerFor(jobServer.PauseJob)),
		atc.UnpauseJob:    validate(pipelineHandlerFactory.HandlerFor(jobServer.UnpauseJob)),

		atc.ListPipelines:   http.HandlerFunc(pipelineServer.ListPipelines),
		atc.OrderPipelines:  validate(http.HandlerFunc(pipelineServer.OrderPipelines)),
		atc.PausePipeline:   validate(pipelineHandlerFactory.HandlerFor(pipelineServer.PausePipeline)),
		atc.UnpausePipeline: validate(pipelineHandlerFactory.HandlerFor(pipelineServer.UnpausePipeline)),

		atc.ListResources:          pipelineHandlerFactory.HandlerFor(resourceServer.ListResources),
		atc.EnableResourceVersion:  validate(pipelineHandlerFactory.HandlerFor(resourceServer.EnableResourceVersion)),
		atc.DisableResourceVersion: validate(pipelineHandlerFactory.HandlerFor(resourceServer.DisableResourceVersion)),
		atc.PauseResource:          validate(pipelineHandlerFactory.HandlerFor(resourceServer.PauseResource)),
		atc.UnpauseResource:        validate(pipelineHandlerFactory.HandlerFor(resourceServer.UnpauseResource)),

		atc.CreatePipe: validate(http.HandlerFunc(pipeServer.CreatePipe)),
		atc.WritePipe:  validate(http.HandlerFunc(pipeServer.WritePipe)),
		atc.ReadPipe:   validate(http.HandlerFunc(pipeServer.ReadPipe)),

		atc.ListWorkers:    validate(http.HandlerFunc(workerServer.ListWorkers)),
		atc.RegisterWorker: validate(http.HandlerFunc(workerServer.RegisterWorker)),

		atc.SetLogLevel: validate(http.HandlerFunc(logLevelServer.SetMinLevel)),
		atc.GetLogLevel: http.HandlerFunc(logLevelServer.GetMinLevel),

		atc.DownloadCLI: http.HandlerFunc(cliServer.Download),
	}

	return rata.NewRouter(atc.Routes, handlers)
}
