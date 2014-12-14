package web

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/atc/scheduler"
	"github.com/concourse/atc/web/abortbuild"
	"github.com/concourse/atc/web/getbuild"
	"github.com/concourse/atc/web/getjob"
	"github.com/concourse/atc/web/getresource"
	"github.com/concourse/atc/web/index"
	"github.com/concourse/atc/web/login"
	"github.com/concourse/atc/web/routes"
	"github.com/concourse/atc/web/triggerbuild"
)

func NewHandler(
	logger lager.Logger,
	validator auth.Validator,
	scheduler *scheduler.Scheduler,
	db db.DB,
	configDB db.ConfigDB,
	templatesDir, publicDir string,
	drain <-chan struct{},
	engine engine.Engine,
) (http.Handler, error) {
	funcs := template.FuncMap{
		"url": templateFuncs{}.url,
	}

	indexTemplate, err := loadTemplate(templatesDir, "index.html", funcs)
	if err != nil {
		return nil, err
	}

	buildTemplate, err := loadTemplate(templatesDir, "build.html", funcs)
	if err != nil {
		return nil, err
	}

	resourceTemplate, err := loadTemplate(templatesDir, "resource.html", funcs)
	if err != nil {
		return nil, err
	}

	jobTemplate, err := loadTemplate(templatesDir, "job.html", funcs)
	if err != nil {
		return nil, err
	}

	absPublicDir, err := filepath.Abs(publicDir)
	if err != nil {
		return nil, err
	}

	handlers := map[string]http.Handler{
		// public
		routes.Index:       index.NewHandler(logger, db, configDB, indexTemplate),
		routes.Public:      http.FileServer(http.Dir(filepath.Dir(absPublicDir))),
		routes.GetJob:      getjob.NewHandler(logger, db, configDB, jobTemplate),
		routes.GetResource: getresource.NewHandler(logger, db, configDB, resourceTemplate),
		routes.GetBuild:    getbuild.NewHandler(logger, db, configDB, buildTemplate),

		// private
		routes.LogIn: auth.Handler{
			Handler:   login.NewHandler(logger),
			Validator: validator,
		},

		routes.TriggerBuild: auth.Handler{
			Handler:   triggerbuild.NewHandler(logger, configDB, scheduler),
			Validator: validator,
		},

		routes.AbortBuild: auth.Handler{
			Handler:   abortbuild.NewHandler(logger, db, engine),
			Validator: validator,
		},
	}

	return rata.NewRouter(routes.Routes, handlers)
}

func loadTemplate(templatesDir, name string, funcs template.FuncMap) (*template.Template, error) {
	return template.New("layout.html").Funcs(funcs).ParseFiles(
		filepath.Join(templatesDir, "layout.html"),
		filepath.Join(templatesDir, name),
	)
}
