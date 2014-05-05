package server

import (
	"net/http"

	"github.com/tedsuo/router"

	"github.com/winston-ci/winston/builder"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/jobs"
	"github.com/winston-ci/winston/server/routes"
	"github.com/winston-ci/winston/server/triggerbuild"
)

type Server struct {
	config config.Config
}

func New(config config.Config, db db.DB, builder builder.Builder) (http.Handler, error) {
	js := make(map[string]jobs.Job)
	for name, config := range config.Jobs {
		js[name] = jobs.Job{
			Name: name,

			BuildConfigPath: config.BuildConfigPath,
		}
	}

	handlers := map[string]http.Handler{
		routes.TriggerBuild: triggerbuild.NewHandler(js, builder),
	}

	return router.NewRouter(routes.Routes, handlers)
}
