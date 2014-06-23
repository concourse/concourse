package server

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/router"

	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/queue"
	"github.com/concourse/atc/server/abortbuild"
	"github.com/concourse/atc/server/getbuild"
	"github.com/concourse/atc/server/index"
	"github.com/concourse/atc/server/routes"
	"github.com/concourse/atc/server/triggerbuild"
)

func New(
	logger lager.Logger,
	config config.Config,
	db db.DB,
	templatesDir, publicDir string,
	peerAddr string,
	queuer queue.Queuer,
) (http.Handler, error) {
	funcs := template.FuncMap{
		"url": templateFuncs{peerAddr}.url,
	}

	indexTemplate, err := loadTemplate(templatesDir, "index.html", funcs)
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
		routes.Index:        index.NewHandler(logger, config.Resources, config.Jobs, db, indexTemplate),
		routes.GetBuild:     getbuild.NewHandler(logger, config.Jobs, db, buildTemplate),
		routes.TriggerBuild: triggerbuild.NewHandler(logger, config.Jobs, queuer),
		routes.AbortBuild:   abortbuild.NewHandler(logger, config.Jobs, db),

		routes.Public: http.FileServer(http.Dir(filepath.Dir(absPublicDir))),
	}

	return router.NewRouter(routes.Routes, handlers)
}

func loadTemplate(templatesDir, name string, funcs template.FuncMap) (*template.Template, error) {
	return template.New("layout.html").Funcs(funcs).ParseFiles(
		filepath.Join(templatesDir, "layout.html"),
		filepath.Join(templatesDir, name),
	)
}
