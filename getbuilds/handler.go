package getbuilds

import (
	"html/template"
	"net/http"

	"github.com/concourse/atc/web"
	"github.com/concourse/go-concourse/concourse"
	"github.com/pivotal-golang/lager"
)

type handler struct {
	logger lager.Logger

	clientFactory web.ClientFactory

	template *template.Template
}

func NewHandler(logger lager.Logger, clientFactory web.ClientFactory, template *template.Template) http.Handler {
	return &handler{
		logger: logger,

		clientFactory: clientFactory,

		template: template,
	}
}

type TemplateData struct {
	Builds []PresentedBuild
}

func FetchTemplateData(client concourse.Client) (TemplateData, error) {
	builds, err := client.AllBuilds()
	if err != nil {
		return TemplateData{}, err
	}

	return TemplateData{
		Builds: PresentBuilds(builds),
	}, nil
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	client := handler.clientFactory.Build(r)

	log := handler.logger.Session("builds")

	templateData, err := FetchTemplateData(client)
	if err != nil {
		log.Error("failed-to-build-template-data", err)
		http.Error(w, "failed to fetch builds", http.StatusInternalServerError)
		return
	}

	err = handler.template.Execute(w, templateData)
	if err != nil {
		log.Fatal("failed-to-build-template", err)
	}
}
