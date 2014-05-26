package index

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
	Jobs []JobStatus
}

type JobStatus struct {
	Job          config.Job
	CurrentBuild builds.Build
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	jobs := []JobStatus{}
	for _, job := range handler.jobs {
		currentBuild, err := handler.db.GetCurrentBuild(job.Name)
		if err != nil {
			currentBuild.Status = builds.StatusPending
		}

		jobs = append(jobs, JobStatus{
			Job:          job,
			CurrentBuild: currentBuild,
		})
	}

	err := handler.template.Execute(w, TemplateData{jobs})
	if err != nil {
		log.Println("failed to execute template:", err)
	}
}
