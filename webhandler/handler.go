package webhandler

import (
	"html/template"
	"net/http"

	"github.com/elazarl/go-bindata-assetfs"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/web"
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

func NewHandler(
	logger lager.Logger,
	wrapper wrappa.Wrappa,
	clientFactory web.ClientFactory,
) (http.Handler, error) {
	tfuncs := &templateFuncs{
		assetIDs: map[string]string{},
	}

	funcs := template.FuncMap{
		"url":          tfuncs.url,
		"asset":        tfuncs.asset,
		"withRedirect": tfuncs.withRedirect,
	}

	indexTemplate, err := loadTemplate("index.html", funcs)
	if err != nil {
		return nil, err
	}

	pipelineTemplate, err := loadTemplateWithPipeline("pipeline.html", funcs)
	if err != nil {
		return nil, err
	}

	buildTemplate, err := loadTemplateWithPipeline("build.html", funcs)
	if err != nil {
		return nil, err
	}

	oldBuildTemplate, err := loadTemplateWithPipeline("old-build.html", funcs)
	if err != nil {
		return nil, err
	}

	buildsTemplate, err := loadTemplateWithoutPipeline("builds/index.html", funcs)
	if err != nil {
		return nil, err
	}

	joblessBuildTemplate, err := loadTemplateWithoutPipeline("builds/show.html", funcs)
	if err != nil {
		return nil, err
	}

	oldJoblessBuildTemplate, err := loadTemplateWithoutPipeline("builds/old-show.html", funcs)
	if err != nil {
		return nil, err
	}

	resourceTemplate, err := loadTemplateWithPipeline("resource.html", funcs)
	if err != nil {
		return nil, err
	}

	jobTemplate, err := loadTemplateWithPipeline("job.html", funcs)
	if err != nil {
		return nil, err
	}

	logInTemplate, err := loadTemplateWithoutPipeline("login.html", funcs)
	if err != nil {
		return nil, err
	}

	publicFS := &assetfs.AssetFS{
		Asset:     web.Asset,
		AssetDir:  web.AssetDir,
		AssetInfo: web.AssetInfo,
	}

	pipelineHandler := pipeline.NewHandler(logger, clientFactory, pipelineTemplate)

	handlers := map[string]http.Handler{
		web.Index:           index.NewHandler(logger, clientFactory, pipelineHandler, indexTemplate),
		web.Pipeline:        pipelineHandler,
		web.Public:          http.FileServer(publicFS),
		web.GetJob:          getjob.NewHandler(logger, clientFactory, jobTemplate),
		web.GetResource:     getresource.NewHandler(logger, clientFactory, resourceTemplate),
		web.GetBuild:        getbuild.NewHandler(logger, clientFactory, buildTemplate, oldBuildTemplate),
		web.GetBuilds:       getbuilds.NewHandler(logger, clientFactory, buildsTemplate),
		web.GetJoblessBuild: getjoblessbuild.NewHandler(logger, clientFactory, joblessBuildTemplate, oldJoblessBuildTemplate),
		web.LogIn:           login.NewHandler(logger, clientFactory, logInTemplate),
		web.BasicAuth:       login.NewBasicAuthHandler(logger),
		web.TriggerBuild:    triggerbuild.NewHandler(logger, clientFactory),
	}

	return rata.NewRouter(web.Routes, wrapper.Wrap(handlers))
}

func loadTemplate(name string, funcs template.FuncMap) (*template.Template, error) {
	src, err := web.Asset("templates/" + name)
	if err != nil {
		return nil, err
	}

	return template.New(name).Funcs(funcs).Parse(string(src))
}

func loadTemplateWithPipeline(name string, funcs template.FuncMap) (*template.Template, error) {
	layout, err := loadTemplate("layouts/with_pipeline.html", funcs)
	if err != nil {
		return nil, err
	}

	templateSrc, err := web.Asset("templates/" + name)
	if err != nil {
		return nil, err
	}

	_, err = layout.New(name).Parse(string(templateSrc))
	if err != nil {
		return nil, err
	}

	return layout, nil
}

func loadTemplateWithoutPipeline(name string, funcs template.FuncMap) (*template.Template, error) {
	layout, err := loadTemplate("layouts/without_pipeline.html", funcs)
	if err != nil {
		return nil, err
	}

	templateSrc, err := web.Asset("templates/" + name)
	if err != nil {
		return nil, err
	}

	_, err = layout.New(name).Parse(string(templateSrc))
	if err != nil {
		return nil, err
	}

	return layout, nil
}
