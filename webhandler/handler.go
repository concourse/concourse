package webhandler

import (
	"html/template"
	"net/http"

	"github.com/elazarl/go-bindata-assetfs"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/web"
	"github.com/concourse/atc/web/authredirect"
	"github.com/concourse/atc/web/basicauth"
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

	buildsTemplate, err := loadTemplateWithoutPipeline("builds/index.html", funcs)
	if err != nil {
		return nil, err
	}

	joblessBuildTemplate, err := loadTemplateWithoutPipeline("builds/show.html", funcs)
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

	basicAuthTemplate, err := loadTemplateWithoutPipeline("basic-auth.html", funcs)
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
		web.Index:                 authredirect.Handler{index.NewHandler(logger, clientFactory, pipelineHandler, indexTemplate)},
		web.Pipeline:              authredirect.Handler{pipelineHandler},
		web.Public:                CacheNearlyForever(http.FileServer(publicFS)),
		web.GetJob:                authredirect.Handler{getjob.NewHandler(logger, clientFactory, jobTemplate)},
		web.GetResource:           authredirect.Handler{getresource.NewHandler(logger, clientFactory, resourceTemplate)},
		web.GetBuild:              authredirect.Handler{getbuild.NewHandler(logger, clientFactory, buildTemplate)},
		web.GetBuilds:             authredirect.Handler{getbuilds.NewHandler(logger, clientFactory, buildsTemplate)},
		web.GetJoblessBuild:       authredirect.Handler{getjoblessbuild.NewHandler(logger, clientFactory, joblessBuildTemplate)},
		web.TriggerBuild:          authredirect.Handler{triggerbuild.NewHandler(logger, clientFactory)},
		web.TeamLogIn:             login.NewHandler(logger, clientFactory, logInTemplate),
		web.LogIn:                 login.NewHandler(logger, clientFactory, logInTemplate),
		web.GetBasicAuthLogIn:     basicauth.NewGetBasicAuthHandler(logger, basicAuthTemplate),
		web.ProcessBasicAuthLogIn: basicauth.NewProcessBasicAuthHandler(logger, clientFactory),
	}

	handler, err := rata.NewRouter(web.Routes, wrapper.Wrap(handlers))
	if err != nil {
		return nil, err
	}

	return authredirect.Tracker{
		Handler: handler,
	}, nil
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
