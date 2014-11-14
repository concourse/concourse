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
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/builder"
)

func NewHandler(
	logger lager.Logger,
	validator auth.Validator,
	buildsDB buildserver.BuildsDB,
	jobsDB jobserver.JobsDB,
	configDB configserver.ConfigDB,
	configValidator configserver.ConfigValidator,
	builder builder.Builder,
	pingInterval time.Duration,
	peerAddr string,
	eventHandlerFactory buildserver.EventHandlerFactory,
	drain <-chan struct{},
) (http.Handler, error) {
	buildServer := buildserver.NewServer(
		logger,
		buildsDB,
		builder,
		pingInterval,
		eventHandlerFactory,
		drain,
	)

	jobServer := jobserver.NewServer(logger, jobsDB, configDB)
	resourceServer := resourceserver.NewServer(logger, configDB)
	pipeServer := pipes.NewServer(logger, peerAddr)

	configServer := configserver.NewServer(logger, configDB, configValidator)

	validate := func(handler http.Handler) http.Handler {
		return auth.Handler{
			Handler:   handler,
			Validator: validator,
		}
	}

	handlers := map[string]http.Handler{
		atc.GetConfig:  validate(http.HandlerFunc(configServer.GetConfig)),
		atc.SaveConfig: validate(http.HandlerFunc(configServer.SaveConfig)),

		atc.CreateBuild: validate(http.HandlerFunc(buildServer.CreateBuild)),
		atc.ListBuilds:  validate(http.HandlerFunc(buildServer.ListBuilds)),
		atc.BuildEvents: validate(http.HandlerFunc(buildServer.BuildEvents)),
		atc.AbortBuild:  validate(http.HandlerFunc(buildServer.AbortBuild)),
		atc.HijackBuild: validate(http.HandlerFunc(buildServer.HijackBuild)),

		atc.ListJobs:      http.HandlerFunc(jobServer.ListJobs),
		atc.GetJob:        validate(http.HandlerFunc(jobServer.GetJob)),
		atc.ListJobBuilds: validate(http.HandlerFunc(jobServer.ListJobBuilds)),
		atc.GetJobBuild:   validate(http.HandlerFunc(jobServer.GetJobBuild)),

		atc.ListResources: http.HandlerFunc(resourceServer.ListResources),

		atc.CreatePipe: validate(http.HandlerFunc(pipeServer.CreatePipe)),
		atc.WritePipe:  validate(http.HandlerFunc(pipeServer.WritePipe)),
		atc.ReadPipe:   validate(http.HandlerFunc(pipeServer.ReadPipe)),
	}

	return rata.NewRouter(atc.Routes, handlers)
}
