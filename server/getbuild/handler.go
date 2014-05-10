package getbuild

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/jobs"
)

type handler struct {
	jobs     map[string]jobs.Job
	db       db.DB
	template *template.Template
}

func NewHandler(jobs map[string]jobs.Job, db db.DB, template *template.Template) http.Handler {
	return &handler{
		jobs:     jobs,
		db:       db,
		template: template,
	}
}

type TemplateData struct {
	Job   jobs.Job
	Build builds.Build
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	job, found := handler.jobs[r.FormValue(":job")]
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	buildID, err := strconv.Atoi(r.FormValue(":build"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	build, err := handler.db.GetBuild(job.Name, buildID)
	if err != nil {
		log.Println("failed to get builds:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = handler.template.Execute(w, TemplateData{
		Job:   job,
		Build: build,
	})
	if err != nil {
		log.Println("failed to execute template:", err)
	}
}
