package api

import (
	"net/http"

	"github.com/tedsuo/router"

	"github.com/winston-ci/winston/api/handler"
	"github.com/winston-ci/winston/api/routes"
	"github.com/winston-ci/winston/db"
)

func New(db db.DB) (http.Handler, error) {
	builds := handler.NewHandler(db)

	handlers := map[string]http.Handler{
		routes.SetResult: http.HandlerFunc(builds.SetResult),

		routes.LogInput:  http.HandlerFunc(builds.LogInput),
		routes.LogOutput: http.HandlerFunc(builds.LogOutput),
	}

	return router.NewRouter(routes.Routes, handlers)
}
