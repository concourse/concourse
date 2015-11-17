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
	Build     atc.Build
	Resources atc.BuildInputsOutputs
}

type handler struct {
	logger lager.Logger

	clientFactory web.ClientFactory

	db       db.DB
	configDB db.ConfigDB

	template *template.Template
}

func NewHandler(logger lager.Logger, clientFactory web.ClientFactory, template *template.Template) http.Handler {
	return &handler{
		logger:        logger,
		clientFactory: clientFactory,
		template:      template,
	}
}

type TemplateData struct {
	Job atc.Job

	Builds     []BuildWithInputsOutputs
	Pagination concourse.Pagination

	GroupStates []group.State

	CurrentBuild *atc.Build
	PipelineName string
}

//go:generate counterfeiter . JobBuildsPaginator

type JobBuildsPaginator interface {
	PaginateJobBuilds(job string, startingJobBuildID int, newerJobBuilds bool) ([]db.Build, pagination.PaginationData, error)
}

var ErrConfigNotFound = errors.New("could not find config")
var ErrJobConfigNotFound = errors.New("could not find job")
var Err = errors.New("could not find job")

func FetchTemplateData(
	pipelineName string,
	client concourse.Client,
	jobName string,
	page concourse.Page,
) (TemplateData, error) {
	pipeline, pipelineFound, err := client.Pipeline(pipelineName)
	if err != nil {
		return TemplateData{}, err
	}

	if !pipelineFound {
		return TemplateData{}, ErrConfigNotFound
	}

	job, jobFound, err := client.Job(pipelineName, jobName)
	if err != nil {
		return TemplateData{}, err
	}

	if !jobFound {
		return TemplateData{}, ErrJobConfigNotFound
	}

	bs, pagination, _, err := client.JobBuilds(pipelineName, jobName, page)
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

	return TemplateData{
		PipelineName: pipelineName,
		Job:          job,

		Builds:     bsr,
		Pagination: pagination,

		GroupStates: group.States(pipeline.Groups, func(g atc.GroupConfig) bool {
			for _, groupJob := range g.Jobs {
				if groupJob == job.Name {
					return true
				}
			}

			return false
		}),

		CurrentBuild: job.FinishedBuild,
	}, nil
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log := handler.logger.Session("job")

	jobName := r.FormValue(":job")
	if len(jobName) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	since, parseErr := strconv.Atoi(r.FormValue("since"))
	if parseErr != nil {
		since = 0
	}

	until, parseErr := strconv.Atoi(r.FormValue("until"))
	if parseErr != nil {
		until = 0
	}

	templateData, err := FetchTemplateData(
		r.FormValue(":pipeline_name"),
		handler.clientFactory.Build(r),
		jobName,
		concourse.Page{
			Since: since,
			Until: until,
			Limit: atc.PaginationWebLimit,
		},
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

	err = handler.template.Execute(w, templateData)
	if err != nil {
		log.Fatal("failed-to-build-template", err, lager.Data{
			"template-data": templateData,
		})
	}
}
