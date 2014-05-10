package index

import (
	"html/template"
	"log"
	"net/http"

	"github.com/winston-ci/winston/jobs"
)

type handler struct {
	jobs     map[string]jobs.Job
	template *template.Template
}

func NewHandler(jobs map[string]jobs.Job, template *template.Template) http.Handler {
	return &handler{
		jobs:     jobs,
		template: template,
	}
}

type TemplateData struct {
	Jobs map[string]jobs.Job
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := handler.template.Execute(w, TemplateData{handler.jobs})
	if err != nil {
		log.Println("failed to execute template:", err)
	}
}
