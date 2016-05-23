package getjob

import (
	"errors"
	"html/template"
	"net/http"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/atc/web"
	"github.com/concourse/atc/web/group"
	"github.com/concourse/go-concourse/concourse"
	"github.com/pivotal-golang/lager"
)

type Handler struct {
	logger lager.Logger

	clientFactory web.ClientFactory

	template *template.Template
}

func NewHandler(logger lager.Logger, clientFactory web.ClientFactory, template *template.Template) *Handler {
	return &Handler{
		logger:        logger,
		clientFactory: clientFactory,
		template:      template,
	}
}

type TemplateData struct {
	JobName string
	Since   int
	Until   int

	GroupStates  []group.State
	PipelineName string
	TeamName     string
}

var ErrConfigNotFound = errors.New("could not find config")
var ErrJobConfigNotFound = errors.New("could not find job")

func FetchTemplateData(
	pipelineName string,
	team concourse.Team,
	jobName string,
	page concourse.Page,
) (TemplateData, error) {
	pipeline, pipelineFound, err := team.Pipeline(pipelineName)
	if err != nil {
		return TemplateData{}, err
	}

	if !pipelineFound {
		return TemplateData{}, ErrConfigNotFound
	}

	_, jobFound, err := team.Job(pipelineName, jobName)
	if err != nil {
		return TemplateData{}, err
	}

	if !jobFound {
		return TemplateData{}, ErrJobConfigNotFound
	}

	return TemplateData{
		TeamName:     team.Name(),
		PipelineName: pipelineName,
		JobName:      jobName,

		Since: page.Since,
		Until: page.Until,

		GroupStates: group.States(pipeline.Groups, func(g atc.GroupConfig) bool {
			for _, groupJob := range g.Jobs {
				if groupJob == jobName {
					return true
				}
			}

			return false
		}),
	}, nil
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	log := handler.logger.Session("job")

	jobName := r.FormValue(":job")
	if len(jobName) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}

	since, parseErr := strconv.Atoi(r.FormValue("since"))
	if parseErr != nil {
		since = 0
	}

	until, parseErr := strconv.Atoi(r.FormValue("until"))
	if parseErr != nil {
		until = 0
	}

	teamName := r.FormValue(":team_name")
	client := handler.clientFactory.Build(r)
	templateData, err := FetchTemplateData(
		r.FormValue(":pipeline_name"),
		client.Team(teamName),
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
		return nil
	case nil:
		break
	default:
		log.Error("failed-to-build-template-data", err, lager.Data{
			"job": jobName,
		})
		return err
	}

	err = handler.template.Execute(w, templateData)
	if err != nil {
		log.Fatal("failed-to-build-template", err, lager.Data{
			"template-data": templateData,
		})

		return err
	}

	return nil
}
