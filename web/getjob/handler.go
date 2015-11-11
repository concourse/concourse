package getjob

import (
	"errors"
	"html/template"
	"net/http"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web"
	"github.com/concourse/atc/web/group"
	"github.com/concourse/atc/web/pagination"
	"github.com/concourse/go-concourse/concourse"
	"github.com/pivotal-golang/lager"
)

type BuildWithInputsOutputs struct {
	Build     db.Build
	Resources atc.BuildInputsOutputs
}

type server struct {
	logger lager.Logger

	clientFactory web.ClientFactory

	db       db.DB
	configDB db.ConfigDB

	template *template.Template
}

func NewServer(logger lager.Logger, clientFactory web.ClientFactory, template *template.Template) *server {
	return &server{
		logger:        logger,
		clientFactory: clientFactory,
		template:      template,
	}
}

type TemplateData struct {
	Job    atc.JobConfig
	DBJob  db.SavedJob
	Builds []BuildWithInputsOutputs

	GroupStates []group.State

	CurrentBuild *db.Build
	PipelineName string

	PaginationData pagination.PaginationData
}

//go:generate counterfeiter . JobDB

type JobDB interface {
	GetJob(string) (db.SavedJob, error)
	GetCurrentBuild(job string) (db.Build, bool, error)
	GetPipelineName() string
}

//go:generate counterfeiter . JobBuildsPaginator

type JobBuildsPaginator interface {
	PaginateJobBuilds(job string, startingJobBuildID int, newerJobBuilds bool) ([]db.Build, pagination.PaginationData, error)
}

var ErrJobConfigNotFound = errors.New("could not find job")
var Err = errors.New("could not find job")

func FetchTemplateData(client concourse.Client, pipelineName string, jobDB JobDB, paginator JobBuildsPaginator, jobName string, startingJobBuildID int, resultsGreaterThanStartingID bool) (TemplateData, error) {

	config, _, found, err := client.PipelineConfig(pipelineName)
	if err != nil {
		return TemplateData{}, err
	}

	if !found {
		return TemplateData{}, ErrJobConfigNotFound
	}

	job, found := config.Jobs.Lookup(jobName)
	if !found {
		return TemplateData{}, ErrJobConfigNotFound
	}

	bs, paginationData, err := paginator.PaginateJobBuilds(job.Name, startingJobBuildID, resultsGreaterThanStartingID)
	if err != nil {
		return TemplateData{}, err
	}

	var bsr []BuildWithInputsOutputs

	for _, build := range bs {
		buildInputsOutputs, _, err := client.BuildResources(build.ID)
		if err != nil {
			return TemplateData{}, err
		}

		bsr = append(bsr, BuildWithInputsOutputs{
			Build:     build,
			Resources: buildInputsOutputs,
		})
	}

	var currentBuild *db.Build

	current, found, err := jobDB.GetCurrentBuild(job.Name)
	if err != nil {
		return TemplateData{}, err
	}

	if found {
		currentBuild = &current
	}

	dbJob, err := jobDB.GetJob(job.Name)
	if err != nil {
		return TemplateData{}, err
	}

	return TemplateData{
		PipelineName:   jobDB.GetPipelineName(),
		Job:            job,
		DBJob:          dbJob,
		Builds:         bsr,
		PaginationData: paginationData,

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

func (server *server) GetJob(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := server.logger.Session("job")

		jobName := r.FormValue(":job")
		if len(jobName) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		startingID, parseErr := strconv.Atoi(r.FormValue("startingID"))
		if parseErr != nil {
			log.Info("cannot-parse-startingID-to-int", lager.Data{"startingID": r.FormValue("startingID")})
			startingID = 0
		}

		resultsGreaterThanStartingID, parseErr := strconv.ParseBool(r.FormValue("resultsGreaterThanStartingID"))
		if parseErr != nil {
			resultsGreaterThanStartingID = false
			log.Info("cannot-parse-resultsGreaterThanStartingID-to-bool", lager.Data{"resultsGreaterThanStartingID": r.FormValue("resultsGreaterThanStartingID")})
		}

		templateData, err := FetchTemplateData(
			server.clientFactory.Build(r),
			r.FormValue(":pipeline_name"),
			pipelineDB,
			Paginator{
				PaginatorDB: pipelineDB,
			},
			jobName,
			startingID,
			resultsGreaterThanStartingID,
		)
		switch err {
		case ErrJobConfigNotFound:
			log.Info("could-not-find-job-in-config", lager.Data{
				"job": jobName,
			})
			w.WriteHeader(http.StatusNotFound)
			return
		case nil:
			break
		default:
			log.Error("failed-to-build-template-data", err, lager.Data{
				"job": jobName,
			})
			http.Error(w, "failed to fetch job", http.StatusInternalServerError)
			return
		}

		err = server.template.Execute(w, templateData)
		if err != nil {
			log.Fatal("failed-to-build-template", err, lager.Data{
				"template-data": templateData,
			})
		}
	})
}
