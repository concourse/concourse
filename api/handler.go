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
	"github.com/concourse/atc/api/pipes"
	"github.com/concourse/atc/api/resourceserver"
	"github.com/concourse/atc/api/workerserver"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/worker"
)

func NewHandler(
	logger lager.Logger,
	validator auth.Validator,

	configDB db.ConfigDB,

	buildsDB buildserver.BuildsDB,
	jobsDB jobserver.JobsDB,
	resourceDB resourceserver.ResourceDB,
	workerDB workerserver.WorkerDB,

	configValidator configserver.ConfigValidator,
	peerAddr string,
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

	jobServer := jobserver.NewServer(logger, jobsDB, configDB)
	resourceServer := resourceserver.NewServer(logger, resourceDB, configDB, validator)
	pipeServer := pipes.NewServer(logger, peerAddr)

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

		atc.ListJobs:      http.HandlerFunc(jobServer.ListJobs),
		atc.GetJob:        http.HandlerFunc(jobServer.GetJob),
		atc.ListJobBuilds: http.HandlerFunc(jobServer.ListJobBuilds),
		atc.GetJobBuild:   http.HandlerFunc(jobServer.GetJobBuild),

		atc.ListResources:          http.HandlerFunc(resourceServer.ListResources),
		atc.EnableResourceVersion:  validate(http.HandlerFunc(resourceServer.EnableResourceVersion)),
		atc.DisableResourceVersion: validate(http.HandlerFunc(resourceServer.DisableResourceVersion)),
		atc.PauseResource:          validate(http.HandlerFunc(resourceServer.PauseResource)),
		atc.UnpauseResource:        validate(http.HandlerFunc(resourceServer.UnpauseResource)),

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
