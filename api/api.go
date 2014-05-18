package api

import (
	"net/http"

	"code.google.com/p/go.net/websocket"
	"github.com/tedsuo/router"

	"github.com/winston-ci/winston/api/handler"
	"github.com/winston-ci/winston/api/routes"
	"github.com/winston-ci/winston/db"
)

func New(db db.DB) (http.Handler, error) {
	builds := handler.NewHandler(db)

	handlers := map[string]http.Handler{
		routes.UpdateBuild: http.HandlerFunc(builds.UpdateBuild),

		routes.LogInput:  websocket.Server{Handler: builds.LogInput},
		routes.LogOutput: websocket.Server{Handler: builds.LogOutput},
	}

	return router.NewRouter(routes.Routes, handlers)
}
