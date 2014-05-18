package index

import (
	"html/template"
	"log"
	"net/http"
	"github.com/winston-ci/winston/config"
)

type handler struct {
	jobs     config.Jobs
	template *template.Template
}

func NewHandler(jobs config.Jobs, template *template.Template) http.Handler {
	return &handler{
		jobs:     jobs,
		template: template,
	}
}

type TemplateData struct {
	Jobs config.Jobs
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := handler.template.Execute(w, TemplateData{handler.jobs})
	if err != nil {
		log.Println("failed to execute template:", err)
	}
}
