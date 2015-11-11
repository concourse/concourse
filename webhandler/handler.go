package webhandler

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
	"github.com/concourse/atc/web"
	"github.com/concourse/atc/web/debug"
	"github.com/concourse/atc/web/getbuild"
	"github.com/concourse/atc/web/getbuilds"
	"github.com/concourse/atc/web/getjob"
	"github.com/concourse/atc/web/getjoblessbuild"
	"github.com/concourse/atc/web/getresource"
	"github.com/concourse/atc/web/index"
	"github.com/concourse/atc/web/login"
	"github.com/concourse/atc/web/pipeline"
	"github.com/concourse/atc/web/triggerbuild"
	"github.com/concourse/atc/wrappa"
)

//go:generate counterfeiter . WebDB

type WebDB interface {
	GetBuild(buildID int) (db.Build, bool, error)
	GetAllBuilds() ([]db.Build, error)

	FindContainersByIdentifier(db.ContainerIdentifier) ([]db.Container, error)
	Workers() ([]db.WorkerInfo, error)
}

func NewHandler(
	logger lager.Logger,
	wrapper wrappa.Wrappa,
	providers auth.Providers,
	basicAuthEnabled bool,
	radarSchedulerFactory pipelines.RadarSchedulerFactory,
	db WebDB,
	pipelineDBFactory db.PipelineDBFactory,
	configDB db.ConfigDB,
	templatesDir,
	publicDir string,
	engine engine.Engine,
	clientFactory web.ClientFactory,
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

	debugTemplate, err := loadTemplateWithoutPipeline(templatesDir, "debug.html", funcs)
	if err != nil {
		return nil, err
	}

	logInTemplate, err := loadTemplateWithoutPipeline(templatesDir, "login.html", funcs)
	if err != nil {
		return nil, err
	}

	absPublicDir, err := filepath.Abs(publicDir)
	if err != nil {
		return nil, err
	}

	jobServer := getjob.NewServer(logger, clientFactory, jobTemplate)
	resourceServer := getresource.NewServer(logger, resourceTemplate)
	pipelineHandler := pipeline.NewHandler(logger, clientFactory, pipelineTemplate)
	buildServer := getbuild.NewServer(logger, buildTemplate)
	triggerBuildServer := triggerbuild.NewServer(logger, radarSchedulerFactory)

	handlers := map[string]http.Handler{
		web.Index:           index.NewHandler(logger, pipelineDBFactory, pipelineHandler, indexTemplate),
		web.Pipeline:        pipelineHandler,
		web.Public:          http.FileServer(http.Dir(filepath.Dir(absPublicDir))),
		web.GetJob:          pipelineHandlerFactory.HandlerFor(jobServer.GetJob),
		web.GetResource:     pipelineHandlerFactory.HandlerFor(resourceServer.GetResource),
		web.GetBuild:        pipelineHandlerFactory.HandlerFor(buildServer.GetBuild),
		web.GetBuilds:       getbuilds.NewHandler(logger, clientFactory, buildsTemplate),
		web.GetJoblessBuild: getjoblessbuild.NewHandler(logger, db, configDB, joblessBuildTemplate),
		web.LogIn:           login.NewHandler(logger, basicAuthEnabled, providers, logInTemplate),
		web.BasicAuth:       login.NewBasicAuthHandler(logger),
		web.TriggerBuild:    pipelineHandlerFactory.HandlerFor(triggerBuildServer.TriggerBuild),
		web.Debug:           debug.NewServer(logger, db, debugTemplate),
	}

	return rata.NewRouter(web.Routes, wrapper.Wrap(handlers))
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
