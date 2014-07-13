package api

import (
	"net/http"

	"code.google.com/p/go.net/websocket"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/api/handler"
	"github.com/concourse/atc/api/routes"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/logfanout"
)

func New(logger lager.Logger, db db.DB, tracker *logfanout.Tracker) (http.Handler, error) {
	builds := handler.NewHandler(logger, db, tracker)

	handlers := map[string]http.Handler{
		routes.UpdateBuild: http.HandlerFunc(builds.UpdateBuild),

		routes.LogInput: websocket.Server{Handler: builds.LogInput},
	}

	return rata.NewRouter(routes.Routes, handlers)
}
