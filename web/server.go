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
	"github.com/concourse/atc/pipelines"
	"github.com/concourse/atc/web/getbuild"
	"github.com/concourse/atc/web/getbuilds"
	"github.com/concourse/atc/web/getjob"
	"github.com/concourse/atc/web/getjoblessbuild"
	"github.com/concourse/atc/web/getresource"
	"github.com/concourse/atc/web/index"
	"github.com/concourse/atc/web/login"
	"github.com/concourse/atc/web/pipeline"
	"github.com/concourse/atc/web/routes"
	"github.com/concourse/atc/web/triggerbuild"
)

func NewHandler(
	logger lager.Logger,
	validator auth.Validator,
	radarSchedulerFactory pipelines.RadarSchedulerFactory,
	db db.DB,
	pipelineDBFactory db.PipelineDBFactory,
	configDB db.ConfigDB,
	templatesDir, publicDir string,
	drain <-chan struct{},
	engine engine.Engine,
) (http.Handler, error) {
	tfuncs := &templateFuncs{
		assetsDir: publicDir,
		assetIDs:  map[string]string{},
	}

	funcs := template.FuncMap{
		"url":   tfuncs.url,
		"asset": tfuncs.asset,
	}

	pipelineHandlerFactory := pipelines.NewHandlerFactory(pipelineDBFactory)

	indexTemplate, err := template.New("index.html").Funcs(funcs).ParseFiles(filepath.Join(templatesDir, "index.html"))
	if err != nil {
		return nil, err
	}

	pipelineTemplate, err := loadTemplate(templatesDir, "pipeline.html", funcs)
	if err != nil {
		return nil, err
	}

	buildTemplate, err := loadTemplate(templatesDir, "build.html", funcs)
	if err != nil {
		return nil, err
	}

	buildsTemplate, err := loadTemplate(templatesDir, filepath.Join("builds", "index.html"), funcs)
	if err != nil {
		return nil, err
	}

	joblessBuildTemplate, err := loadTemplate(templatesDir, filepath.Join("builds", "show.html"), funcs)
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

	jobServer := getjob.NewServer(logger, jobTemplate)
	resourceServer := getresource.NewServer(logger, resourceTemplate, validator)
	pipelineServer := pipeline.NewServer(logger, pipelineTemplate)
	buildServer := getbuild.NewServer(logger, buildTemplate)
	triggerBuildServer := triggerbuild.NewServer(logger, radarSchedulerFactory)

	handlers := map[string]http.Handler{
		// public
		routes.Index:    index.NewHandler(logger, pipelineDBFactory, pipelineServer.GetPipeline, indexTemplate),
		routes.Pipeline: pipelineHandlerFactory.HandlerFor(pipelineServer.GetPipeline),
		routes.Public:   http.FileServer(http.Dir(filepath.Dir(absPublicDir))),

		routes.GetJob: pipelineHandlerFactory.HandlerFor(jobServer.GetJob),

		routes.GetResource:     pipelineHandlerFactory.HandlerFor(resourceServer.GetResource),
		routes.GetBuild:        pipelineHandlerFactory.HandlerFor(buildServer.GetBuild),
		routes.GetBuilds:       getbuilds.NewHandler(logger, db, configDB, buildsTemplate),
		routes.GetJoblessBuild: getjoblessbuild.NewHandler(logger, db, configDB, joblessBuildTemplate),

		// private
		routes.LogIn: auth.Handler{
			Handler:   login.NewHandler(logger),
			Validator: validator,
		},

		routes.TriggerBuild: auth.Handler{
			Handler:   pipelineHandlerFactory.HandlerFor(triggerBuildServer.TriggerBuild),
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
