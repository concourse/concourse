package api

import (
	"net/http"
	"time"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/api/buildserver"
	"github.com/concourse/atc/api/pipes"
	"github.com/concourse/atc/builder"
	"github.com/concourse/atc/logfanout"
)

func NewHandler(
	logger lager.Logger,
	db buildserver.BuildsDB,
	builder builder.Builder,
	tracker *logfanout.Tracker,
	pingInterval time.Duration,
	peerAddr string,
) (http.Handler, error) {
	buildsServer := buildserver.NewServer(logger, db, builder, tracker, pingInterval)
	pipeServer := pipes.NewServer(logger, peerAddr)

	handlers := map[string]http.Handler{
		CreateBuild: http.HandlerFunc(buildsServer.CreateBuild),
		BuildEvents: http.HandlerFunc(buildsServer.BuildEvents),
		AbortBuild:  http.HandlerFunc(buildsServer.AbortBuild),
		HijackBuild: http.HandlerFunc(buildsServer.HijackBuild),

		CreatePipe: http.HandlerFunc(pipeServer.CreatePipe),
		WritePipe:  http.HandlerFunc(pipeServer.WritePipe),
		ReadPipe:   http.HandlerFunc(pipeServer.ReadPipe),
	}

	return rata.NewRouter(Routes, handlers)
}
