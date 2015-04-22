package getjob

import (
	"errors"
	"html/template"
	"log"
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
	DBJob  db.Job
	Builds []db.Build

	GroupStates []group.State

	CurrentBuild db.Build
}

//go:generate counterfeiter . JobDB

type JobDB interface {
	GetJob(string) (db.Job, error)
	GetAllJobBuilds(job string) ([]db.Build, error)
	GetCurrentBuild(job string) (db.Build, error)
}

var ErrJobConfigNotFound = errors.New("could not find job")
var Err = errors.New("could not find job")

func FetchTemplateData(jobDB JobDB, configDB db.ConfigDB, jobName string) (TemplateData, error) {
	config, _, err := configDB.GetConfig(atc.DefaultPipelineName)
	if err != nil {
		return TemplateData{}, err
	}

	job, found := config.Jobs.Lookup(jobName)
	if !found {
		return TemplateData{}, ErrJobConfigNotFound
	}

	bs, err := jobDB.GetAllJobBuilds(job.Name)
	if err != nil {
		return TemplateData{}, err
	}

	currentBuild, err := jobDB.GetCurrentBuild(job.Name)
	if err != nil {
		currentBuild.Status = db.StatusPending
	}

	dbJob, err := jobDB.GetJob(job.Name)
	if err != nil {
		return TemplateData{}, err
	}

	return TemplateData{
		Job:    job,
		DBJob:  dbJob,
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
	}, nil
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	jobName := r.FormValue(":job")
	if len(jobName) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	templateData, err := FetchTemplateData(handler.db, handler.configDB, jobName)
	switch err {
	case ErrJobConfigNotFound:
		handler.logger.Error("could-not-find-job-in-config", ErrJobConfigNotFound, lager.Data{
			"job": jobName,
		})
		w.WriteHeader(http.StatusNotFound)
		return
	case nil:
		break
	default:
		handler.logger.Error("failed-to-build-template-data", err, lager.Data{
			"job": jobName,
		})
		http.Error(w, "failed to fetch job", http.StatusInternalServerError)
		return
	}

	err = handler.template.Execute(w, templateData)
	if err != nil {
		log.Fatal("failed-to-task-template", err, lager.Data{
			"template-data": templateData,
		})
	}
}
