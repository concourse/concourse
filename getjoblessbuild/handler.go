package getjoblessbuild

import (
	"errors"
	"html/template"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/web"
	"github.com/concourse/go-concourse/concourse"
	"github.com/pivotal-golang/lager"
)

type handler struct {
	logger        lager.Logger
	clientFactory web.ClientFactory
	template      *template.Template
	oldTemplate   *template.Template
}

func NewHandler(logger lager.Logger, clientFactory web.ClientFactory, template *template.Template, oldTemplate *template.Template) http.Handler {
	return &handler{
		logger:        logger,
		clientFactory: clientFactory,
		template:      template,
		oldTemplate:   oldTemplate,
	}
}

type TemplateData struct {
	Build atc.Build
}

var ErrBuildNotFound = errors.New("build not found")

func FetchTemplateData(buildID string, client concourse.Client) (TemplateData, error) {
	build, found, err := client.Build(buildID)
	if err != nil {
		return TemplateData{}, err
	}

	if !found {
		return TemplateData{}, ErrBuildNotFound
	}

	return TemplateData{Build: build}, nil
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log := handler.logger.Session("jobless-build")

	templateData, err := FetchTemplateData(
		r.FormValue(":build_id"),
		handler.clientFactory.Build(r),
	)

	if err == ErrBuildNotFound {
		log.Info("build-not-found")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err != nil {
		log.Error("failed-to-build-template-data", err)
		http.Error(w, "failed to fetch builds", http.StatusInternalServerError)
		return
	}

	err = handler.template.Execute(w, templateData)
	if err != nil {
		log.Fatal("failed-to-execute-template", err)
	}
}
