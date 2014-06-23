package getbuild

import (
	"html/template"
	"net/http"
	"sort"
	"strconv"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

type handler struct {
	logger lager.Logger

	jobs     config.Jobs
	db       db.DB
	template *template.Template
}

func NewHandler(logger lager.Logger, jobs config.Jobs, db db.DB, template *template.Template) http.Handler {
	return &handler{
		logger: logger,

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

	log := handler.logger.Session("get-build", lager.Data{
		"job":   job.Name,
		"build": buildID,
	})

	build, err := handler.db.GetBuild(job.Name, buildID)
	if err != nil {
		log.Error("get-build-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	bs, err := handler.db.Builds(job.Name)
	if err != nil {
		log.Error("get-all-builds-failed", err)
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

	templateData := TemplateData{
		Job:       job,
		Build:     build,
		Builds:    bs,
		Abortable: abortable,
	}

	err = handler.template.Execute(w, templateData)
	if err != nil {
		log.Fatal("failed-to-execute-template", err, lager.Data{
			"template-data": templateData,
		})
	}
}
