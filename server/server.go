package server

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/tedsuo/router"
	"github.com/winston-ci/winston/builder"
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

func New(
	config config.Config,
	db db.DB,
	templatesDir, publicDir string,
	peerAddr string,
	builder builder.Builder,
) (http.Handler, error) {
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
		inputs := []resources.Resource{}

		for rname, _ := range config.Inputs {
			resource, found := rs[rname]
			if !found {
				return nil, fmt.Errorf("unknown input in %s: %s", name, rname)
			}

			inputs = append(inputs, resource)
		}

		js[name] = jobs.Job{
			Name: name,

			Privileged: config.Privileged,

			BuildConfigPath: config.BuildConfigPath,

			Inputs: inputs,
		}
	}

	funcs := template.FuncMap{
		"url": templateFuncs{peerAddr}.url,
	}

	indexTemplate, err := loadTemplate(templatesDir, "index.html", funcs)
	if err != nil {
		return nil, err
	}

	jobTemplate, err := loadTemplate(templatesDir, "job.html", funcs)
	if err != nil {
		return nil, err
	}

	buildTemplate, err := loadTemplate(templatesDir, "build.html", funcs)
	if err != nil {
		return nil, err
	}

	absPublicDir, err := filepath.Abs(publicDir)
	if err != nil {
		return nil, err
	}

	handlers := map[string]http.Handler{
		routes.Index:        index.NewHandler(js, indexTemplate),
		routes.GetJob:       getjob.NewHandler(js, db, jobTemplate),
		routes.GetBuild:     getbuild.NewHandler(js, db, buildTemplate),
		routes.TriggerBuild: triggerbuild.NewHandler(js, builder),
		routes.Public:       http.FileServer(http.Dir(filepath.Dir(absPublicDir))),
	}

	return router.NewRouter(routes.Routes, handlers)
}

func loadTemplate(templatesDir, name string, funcs template.FuncMap) (*template.Template, error) {
	return template.New("layout.html").Funcs(funcs).ParseFiles(
		filepath.Join(templatesDir, "layout.html"),
		filepath.Join(templatesDir, name),
	)
}
