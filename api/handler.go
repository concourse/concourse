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
	"github.com/concourse/atc/builder"
)

func NewHandler(
	logger lager.Logger,
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

	jobServer := jobserver.NewServer(logger, jobsDB)
	pipeServer := pipes.NewServer(logger, peerAddr)

	configServer := configserver.NewServer(logger, configDB, configValidator)

	handlers := map[string]http.Handler{
		atc.GetConfig:  http.HandlerFunc(configServer.GetConfig),
		atc.SaveConfig: http.HandlerFunc(configServer.SaveConfig),

		atc.CreateBuild: http.HandlerFunc(buildServer.CreateBuild),
		atc.ListBuilds:  http.HandlerFunc(buildServer.ListBuilds),
		atc.BuildEvents: http.HandlerFunc(buildServer.BuildEvents),
		atc.AbortBuild:  http.HandlerFunc(buildServer.AbortBuild),
		atc.HijackBuild: http.HandlerFunc(buildServer.HijackBuild),

		atc.GetJob:        http.HandlerFunc(jobServer.GetJob),
		atc.ListJobBuilds: http.HandlerFunc(jobServer.ListJobBuilds),
		atc.GetJobBuild:   http.HandlerFunc(jobServer.GetJobBuild),

		atc.CreatePipe: http.HandlerFunc(pipeServer.CreatePipe),
		atc.WritePipe:  http.HandlerFunc(pipeServer.WritePipe),
		atc.ReadPipe:   http.HandlerFunc(pipeServer.ReadPipe),
	}

	return rata.NewRouter(atc.Routes, handlers)
}
