package api

import (
	"net/http"
	"time"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/buildserver"
	"github.com/concourse/atc/api/configserver"
	"github.com/concourse/atc/api/jobserver"
	"github.com/concourse/atc/api/pipes"
	"github.com/concourse/atc/api/resourceserver"
	"github.com/concourse/atc/api/workerserver"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/engine"
)

func NewHandler(
	logger lager.Logger,
	validator auth.Validator,

	buildsDB buildserver.BuildsDB,
	buildserverConfigDB buildserver.ConfigDB,

	configDB configserver.ConfigDB,

	jobsDB jobserver.JobsDB,
	jobsConfigDB jobserver.ConfigDB,

	resourceDB resourceserver.ResourceDB,
	resourceConfigDB resourceserver.ConfigDB,

	workerDB workerserver.WorkerDB,

	configValidator configserver.ConfigValidator,
	pingInterval time.Duration,
	peerAddr string,
	eventHandlerFactory buildserver.EventHandlerFactory,
	drain <-chan struct{},

	engine engine.Engine,
) (http.Handler, error) {
	buildServer := buildserver.NewServer(
		logger,
		engine,
		buildsDB,
		buildserverConfigDB,
		pingInterval,
		eventHandlerFactory,
		drain,
		validator,
	)

	jobServer := jobserver.NewServer(logger, jobsDB, jobsConfigDB)
	resourceServer := resourceserver.NewServer(logger, resourceDB, resourceConfigDB)
	pipeServer := pipes.NewServer(logger, peerAddr)

	configServer := configserver.NewServer(logger, configDB, configValidator)

	workerServer := workerserver.NewServer(logger, workerDB)

	validate := func(handler http.Handler) http.Handler {
		return auth.Handler{
			Handler:   handler,
			Validator: validator,
		}
	}

	handlers := map[string]http.Handler{
		atc.GetConfig:  validate(http.HandlerFunc(configServer.GetConfig)),
		atc.SaveConfig: validate(http.HandlerFunc(configServer.SaveConfig)),

		atc.ListBuilds:  http.HandlerFunc(buildServer.ListBuilds),
		atc.CreateBuild: validate(http.HandlerFunc(buildServer.CreateBuild)),
		atc.BuildEvents: http.HandlerFunc(buildServer.BuildEvents),
		atc.AbortBuild:  validate(http.HandlerFunc(buildServer.AbortBuild)),
		atc.HijackBuild: validate(http.HandlerFunc(buildServer.HijackBuild)),

		atc.ListJobs:      http.HandlerFunc(jobServer.ListJobs),
		atc.GetJob:        http.HandlerFunc(jobServer.GetJob),
		atc.ListJobBuilds: http.HandlerFunc(jobServer.ListJobBuilds),
		atc.GetJobBuild:   http.HandlerFunc(jobServer.GetJobBuild),

		atc.ListResources:          http.HandlerFunc(resourceServer.ListResources),
		atc.EnableResourceVersion:  validate(http.HandlerFunc(resourceServer.EnableResourceVersion)),
		atc.DisableResourceVersion: validate(http.HandlerFunc(resourceServer.DisableResourceVersion)),

		atc.CreatePipe: validate(http.HandlerFunc(pipeServer.CreatePipe)),
		atc.WritePipe:  validate(http.HandlerFunc(pipeServer.WritePipe)),
		atc.ReadPipe:   validate(http.HandlerFunc(pipeServer.ReadPipe)),

		atc.ListWorkers:    validate(http.HandlerFunc(workerServer.ListWorkers)),
		atc.RegisterWorker: validate(http.HandlerFunc(workerServer.RegisterWorker)),
	}

	return rata.NewRouter(atc.Routes, handlers)
}
