package getjob

import (
	"html/template"
	"log"
	"net/http"

	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/db"
)

type handler struct {
	jobs     config.Jobs
	db       db.DB
	template *template.Template
}

func NewHandler(jobs config.Jobs, db db.DB, template *template.Template) http.Handler {
	return &handler{
		jobs:     jobs,
		db:       db,
		template: template,
	}
}

type TemplateData struct {
	Job    config.Job
	Builds []builds.Build
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	job, found := handler.jobs.Lookup(r.FormValue(":job"))
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	builds, err := handler.db.Builds(job.Name)
	if err != nil {
		log.Println("failed to get builds:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = handler.template.Execute(w, TemplateData{
		Job:    job,
		Builds: builds,
	})
	if err != nil {
		log.Println("failed to execute template:", err)
	}
}
