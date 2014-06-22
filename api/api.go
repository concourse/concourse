package api

import (
	"net/http"

	"code.google.com/p/go.net/websocket"
	"github.com/tedsuo/router"

	"github.com/concourse/atc/api/drainer"
	"github.com/concourse/atc/api/handler"
	"github.com/concourse/atc/api/routes"
	"github.com/concourse/atc/db"
)

func New(db db.DB, drain *drainer.Drainer) (http.Handler, error) {
	builds := handler.NewHandler(db, drain)

	handlers := map[string]http.Handler{
		routes.UpdateBuild: http.HandlerFunc(builds.UpdateBuild),

		routes.LogInput:  websocket.Server{Handler: builds.LogInput},
		routes.LogOutput: websocket.Server{Handler: builds.LogOutput},
	}

	return router.NewRouter(routes.Routes, handlers)
}
