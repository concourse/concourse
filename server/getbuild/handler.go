package getbuild

import (
	"html/template"
	"log"
	"net/http"
	"sort"
	"strconv"

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
	Build  builds.Build
	Builds []builds.Build

	Abortable bool
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	job, found := handler.jobs.Lookup(r.FormValue(":job"))
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
		log.Println("failed to get build:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	bs, err := handler.db.Builds(job.Name)
	if err != nil {
		log.Println("failed to get builds:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	sort.Sort(sort.Reverse(builds.ByID(bs)))

	var abortable bool
	switch build.Status {
	case builds.StatusPending, builds.StatusStarted:
		abortable = true
	default:
		abortable = false
	}

	err = handler.template.Execute(w, TemplateData{
		Job:       job,
		Build:     build,
		Builds:    bs,
		Abortable: abortable,
	})
	if err != nil {
		log.Println("failed to execute template:", err)
	}
}
