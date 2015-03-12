package getjob

import (
	"html/template"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/group"
	"github.com/pivotal-golang/lager"
)

type handler struct {
	logger lager.Logger

	db       db.DB
	configDB db.ConfigDB

	template *template.Template
}

func NewHandler(logger lager.Logger, db db.DB, configDB db.ConfigDB, template *template.Template) http.Handler {
	return &handler{
		logger: logger,

		db:       db,
		configDB: configDB,

		template: template,
	}
}

type TemplateData struct {
	Job    atc.JobConfig
	Builds []db.Build

	GroupStates []group.State

	CurrentBuild db.Build
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	jobName := r.FormValue(":job")
	if len(jobName) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	config, _, err := handler.configDB.GetConfig()
	if err != nil {
		handler.logger.Error("failed-to-load-config", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	job, found := config.Jobs.Lookup(jobName)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	log := handler.logger.Session("get-job", lager.Data{
		"job": job.Name,
	})

	bs, err := handler.db.GetAllJobBuilds(jobName)
	if err != nil {
		log.Error("get-all-builds-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	currentBuild, err := handler.db.GetCurrentBuild(job.Name)
	if err != nil {
		currentBuild.Status = db.StatusPending
	}

	templateData := TemplateData{
		Job:    job,
		Builds: bs,

		GroupStates: group.States(config.Groups, func(g atc.GroupConfig) bool {
			for _, groupJob := range g.Jobs {
				if groupJob == job.Name {
					return true
				}
			}

			return false
		}),

		CurrentBuild: currentBuild,
	}

	err = handler.template.Execute(w, templateData)
	if err != nil {
		log.Fatal("failed-to-task-template", err, lager.Data{
			"template-data": templateData,
		})
	}
}
