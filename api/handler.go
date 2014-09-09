package api

import (
	"net/http"
	"time"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/api/buildserver"
	"github.com/concourse/atc/api/jobserver"
	"github.com/concourse/atc/api/pipes"
	"github.com/concourse/atc/api/routes"
	"github.com/concourse/atc/builder"
	"github.com/concourse/atc/logfanout"
)

func NewHandler(
	logger lager.Logger,
	buildsDB buildserver.BuildsDB,
	jobsDB jobserver.JobsDB,
	builder builder.Builder,
	tracker *logfanout.Tracker,
	pingInterval time.Duration,
	peerAddr string,
) (http.Handler, error) {
	buildServer := buildserver.NewServer(logger, buildsDB, builder, tracker, pingInterval)
	jobServer := jobserver.NewServer(logger, jobsDB)
	pipeServer := pipes.NewServer(logger, peerAddr)

	handlers := map[string]http.Handler{
		routes.CreateBuild: http.HandlerFunc(buildServer.CreateBuild),
		routes.ListBuilds:  http.HandlerFunc(buildServer.ListBuilds),
		routes.BuildEvents: http.HandlerFunc(buildServer.BuildEvents),
		routes.AbortBuild:  http.HandlerFunc(buildServer.AbortBuild),
		routes.HijackBuild: http.HandlerFunc(buildServer.HijackBuild),

		routes.GetJob:        http.HandlerFunc(jobServer.GetJob),
		routes.ListJobBuilds: http.HandlerFunc(jobServer.ListJobBuilds),
		routes.GetJobBuild:   http.HandlerFunc(jobServer.GetJobBuild),

		routes.CreatePipe: http.HandlerFunc(pipeServer.CreatePipe),
		routes.WritePipe:  http.HandlerFunc(pipeServer.WritePipe),
		routes.ReadPipe:   http.HandlerFunc(pipeServer.ReadPipe),
	}

	return rata.NewRouter(routes.Routes, handlers)
}
