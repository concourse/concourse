package index

import (
	"html/template"
	"net/http"
	"net/url"

	"github.com/concourse/atc/web"
	"github.com/concourse/atc/web/pipeline"
	"github.com/pivotal-golang/lager"
)

type TemplateData struct{}

type Handler struct {
	logger           lager.Logger
	clientFactory    web.ClientFactory
	pipelineHandler  *pipeline.Handler
	noBuildsTemplate *template.Template
}

func NewHandler(
	logger lager.Logger,
	clientFactory web.ClientFactory,
	pipelineHandler *pipeline.Handler,
	noBuildsTemplate *template.Template,
) *Handler {
	return &Handler{
		logger:           logger,
		clientFactory:    clientFactory,
		pipelineHandler:  pipelineHandler,
		noBuildsTemplate: noBuildsTemplate,
	}
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	log := handler.logger.Session("index")

	client := handler.clientFactory.Build(r)

	pipelines, err := client.ListPipelines()
	if err != nil {
		log.Error("failed-to-list-pipelines", err)
		return err
	}

	if len(pipelines) == 0 {
		err := handler.noBuildsTemplate.Execute(w, TemplateData{})
		if err != nil {
			log.Fatal("failed-to-build-template", err, lager.Data{})
			return err
		}

		return nil
	}

	if r.Form == nil {
		r.Form = url.Values{}
	}

	r.Form[":team_name"] = []string{pipelines[0].TeamName}
	r.Form[":pipeline"] = []string{pipelines[0].Name}

	return handler.pipelineHandler.ServeHTTP(w, r)
}
