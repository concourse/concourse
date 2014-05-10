package server

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/tedsuo/router"

	"github.com/winston-ci/winston/builder"
	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/jobs"
	"github.com/winston-ci/winston/resources"
	"github.com/winston-ci/winston/server/getbuild"
	"github.com/winston-ci/winston/server/getjob"
	"github.com/winston-ci/winston/server/index"
	"github.com/winston-ci/winston/server/routes"
	"github.com/winston-ci/winston/server/triggerbuild"
)

type Server struct {
	config config.Config
}

func New(config config.Config, db db.DB, templatesDir string, builder builder.Builder) (http.Handler, error) {
	js := make(map[string]jobs.Job)
	rs := make(map[string]resources.Resource)

	for name, config := range config.Resources {
		rs[name] = resources.Resource{
			Name: name,

			Type: config.Type,
			URI:  config.URI,
		}
	}

	for name, config := range config.Jobs {
		js[name] = jobs.Job{
			Name: name,

			BuildConfigPath: config.BuildConfigPath,
		}
	}

	indexTemplate, err := loadTemplate(templatesDir, "index.html")
	if err != nil {
		return nil, err
	}

	jobTemplate, err := loadTemplate(templatesDir, "job.html")
	if err != nil {
		return nil, err
	}

	buildTemplate, err := loadTemplate(templatesDir, "build.html")
	if err != nil {
		return nil, err
	}

	handlers := map[string]http.Handler{
		routes.Index:        index.NewHandler(js, indexTemplate),
		routes.GetJob:       getjob.NewHandler(js, db, jobTemplate),
		routes.GetBuild:     getbuild.NewHandler(js, db, buildTemplate),
		routes.TriggerBuild: triggerbuild.NewHandler(js, builder),
	}

	return router.NewRouter(routes.Routes, handlers)
}

func urlFor(handler string, args ...interface{}) (string, error) {
	switch handler {
	case routes.TriggerBuild, routes.GetJob:
		return routes.Routes.PathForHandler(handler, router.Params{
			"job": args[0].(jobs.Job).Name,
		})
	case routes.GetBuild:
		return routes.Routes.PathForHandler(handler, router.Params{
			"job":   args[0].(jobs.Job).Name,
			"build": fmt.Sprintf("%d", args[1].(builds.Build).ID),
		})
	default:
		return "", fmt.Errorf("unknown route: %s", handler)
	}
}

func loadTemplate(templatesDir, name string) (*template.Template, error) {
	return template.New("layout.html").Funcs(template.FuncMap{
		"url": urlFor,
	}).ParseFiles(
		filepath.Join(templatesDir, "layout.html"),
		filepath.Join(templatesDir, name),
	)
}
