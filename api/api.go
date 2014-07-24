package api

import (
	"net/http"

	"code.google.com/p/go.net/websocket"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/api/handler"
	"github.com/concourse/atc/api/routes"
	"github.com/concourse/atc/logfanout"
)

func New(logger lager.Logger, buildDB handler.BuildDB, logTracker *logfanout.Tracker) (http.Handler, error) {
	builds := handler.NewHandler(logger, buildDB, logTracker)

	handlers := map[string]http.Handler{
		routes.UpdateBuild: http.HandlerFunc(builds.UpdateBuild),

		routes.LogInput: websocket.Server{Handler: builds.LogInput},
	}

	return rata.NewRouter(routes.Routes, handlers)
}
