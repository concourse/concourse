package api

import (
	"net/http"
	"time"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/api/buildserver"
	"github.com/concourse/atc/api/jobserver"
	"github.com/concourse/atc/api/pipes"
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
		CreateBuild: http.HandlerFunc(buildServer.CreateBuild),
		ListBuilds:  http.HandlerFunc(buildServer.ListBuilds),
		BuildEvents: http.HandlerFunc(buildServer.BuildEvents),
		AbortBuild:  http.HandlerFunc(buildServer.AbortBuild),
		HijackBuild: http.HandlerFunc(buildServer.HijackBuild),

		GetJobBuild: http.HandlerFunc(jobServer.GetJobBuild),

		CreatePipe: http.HandlerFunc(pipeServer.CreatePipe),
		WritePipe:  http.HandlerFunc(pipeServer.WritePipe),
		ReadPipe:   http.HandlerFunc(pipeServer.ReadPipe),
	}

	return rata.NewRouter(Routes, handlers)
}
