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
	"github.com/concourse/atc/metric"
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

//go:generate counterfeiter . WebDB

type WebDB interface {
	GetBuild(buildID int) (db.Build, bool, error)
	GetAllBuilds() ([]db.Build, error)
}

func NewHandler(
	logger lager.Logger,
	validator auth.Validator,
	radarSchedulerFactory pipelines.RadarSchedulerFactory,
	db WebDB,
	pipelineDBFactory db.PipelineDBFactory,
	configDB db.ConfigDB,
	templatesDir, publicDir string,
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

	pipelineTemplate, err := loadTemplateWithPipeline(templatesDir, "pipeline.html", funcs)
	if err != nil {
		return nil, err
	}

	buildTemplate, err := loadTemplateWithPipeline(templatesDir, "build.html", funcs)
	if err != nil {
		return nil, err
	}

	buildsTemplate, err := loadTemplateWithoutPipeline(templatesDir, filepath.Join("builds", "index.html"), funcs)
	if err != nil {
		return nil, err
	}

	joblessBuildTemplate, err := loadTemplateWithoutPipeline(templatesDir, filepath.Join("builds", "show.html"), funcs)
	if err != nil {
		return nil, err
	}

	resourceTemplate, err := loadTemplateWithPipeline(templatesDir, "resource.html", funcs)
	if err != nil {
		return nil, err
	}

	jobTemplate, err := loadTemplateWithPipeline(templatesDir, "job.html", funcs)
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

	for route, handler := range handlers {
		handlers[route] = metric.WrapHandler(route, handler, logger)
	}

	return rata.NewRouter(routes.Routes, handlers)
}

func loadTemplateWithPipeline(templatesDir, name string, funcs template.FuncMap) (*template.Template, error) {
	return template.New("with_pipeline.html").Funcs(funcs).ParseFiles(
		filepath.Join(templatesDir, "layouts", "with_pipeline.html"),
		filepath.Join(templatesDir, name),
	)
}

func loadTemplateWithoutPipeline(templatesDir, name string, funcs template.FuncMap) (*template.Template, error) {
	return template.New("without_pipeline.html").Funcs(funcs).ParseFiles(
		filepath.Join(templatesDir, "layouts", "without_pipeline.html"),
		filepath.Join(templatesDir, name),
	)
}
