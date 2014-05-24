package server

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/tedsuo/router"

	"github.com/winston-ci/winston/builder"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/server/getbuild"
	"github.com/winston-ci/winston/server/getjob"
	"github.com/winston-ci/winston/server/index"
	"github.com/winston-ci/winston/server/routes"
	"github.com/winston-ci/winston/server/triggerbuild"
)

func New(
	config config.Config,
	db db.DB,
	templatesDir, publicDir string,
	peerAddr string,
	builder builder.Builder,
) (http.Handler, error) {
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
		routes.Index:        index.NewHandler(config.Jobs, indexTemplate),
		routes.GetJob:       getjob.NewHandler(config.Jobs, db, jobTemplate),
		routes.GetBuild:     getbuild.NewHandler(config.Jobs, db, buildTemplate),
		routes.TriggerBuild: triggerbuild.NewHandler(config.Jobs, builder),
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
