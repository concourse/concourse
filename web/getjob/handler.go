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

type BuildWithInputsOutputs struct {
	Build   db.Build
	Inputs  []db.BuildInput
	Outputs []db.BuildOutput
}

type server struct {
	logger lager.Logger

	db       db.DB
	configDB db.ConfigDB

	template *template.Template
}

func NewServer(logger lager.Logger, template *template.Template) *server {
	return &server{
		logger: logger,

		template: template,
	}
}

type TemplateData struct {
	Job    atc.JobConfig
	DBJob  db.SavedJob
	Builds []BuildWithInputsOutputs

	GroupStates []group.State

	CurrentBuild db.Build
	PipelineName string
}

//go:generate counterfeiter . JobDB

type JobDB interface {
	GetConfig() (atc.Config, db.ConfigVersion, error)
	GetJob(string) (db.SavedJob, error)
	GetAllJobBuilds(job string) ([]db.Build, error)
	GetCurrentBuild(job string) (db.Build, error)
	GetPipelineName() string
	GetBuildResources(buildID int) ([]db.BuildInput, []db.BuildOutput, error)
}

var ErrJobConfigNotFound = errors.New("could not find job")
var Err = errors.New("could not find job")

func FetchTemplateData(jobDB JobDB, jobName string) (TemplateData, error) {
	config, _, err := jobDB.GetConfig()
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

	var bsr []BuildWithInputsOutputs

	for _, build := range bs {
		inputs, outputs, err := jobDB.GetBuildResources(build.ID)
		if err != nil {
			return TemplateData{}, err
		}

		bsr = append(bsr, BuildWithInputsOutputs{
			Build:   build,
			Inputs:  inputs,
			Outputs: outputs,
		})
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
		Builds: bsr,

		GroupStates: group.States(config.Groups, func(g atc.GroupConfig) bool {
			for _, groupJob := range g.Jobs {
				if groupJob == job.Name {
					return true
				}
			}

			return false
		}),

		CurrentBuild: currentBuild,
		PipelineName: jobDB.GetPipelineName(),
	}, nil
}

func (server *server) GetJob(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobName := r.FormValue(":job")
		if len(jobName) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		templateData, err := FetchTemplateData(pipelineDB, jobName)
		switch err {
		case ErrJobConfigNotFound:
			server.logger.Error("could-not-find-job-in-config", ErrJobConfigNotFound, lager.Data{
				"job": jobName,
			})
			w.WriteHeader(http.StatusNotFound)
			return
		case nil:
			break
		default:
			server.logger.Error("failed-to-build-template-data", err, lager.Data{
				"job": jobName,
			})
			http.Error(w, "failed to fetch job", http.StatusInternalServerError)
			return
		}

		err = server.template.Execute(w, templateData)
		if err != nil {
			log.Fatal("failed-to-task-template", err, lager.Data{
				"template-data": templateData,
			})
		}
	})
}
