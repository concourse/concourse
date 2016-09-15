package getjob

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/concourse/atc/web"

	"code.cloudfoundry.org/lager"
)

type Handler struct {
	logger        lager.Logger
	clientFactory web.ClientFactory
	template      *template.Template
}

func NewHandler(logger lager.Logger, clientFactory web.ClientFactory, template *template.Template) *Handler {
	return &Handler{
		logger:        logger,
		clientFactory: clientFactory,
		template:      template,
	}
}

type TemplateData struct {
	TeamName     string
	PipelineName string
	JobName      string

	Since       int
	Until       int
	QueryGroups []string
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	log := handler.logger.Session("job")

	jobName := r.FormValue(":job")
	if len(jobName) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}

	teamName := r.FormValue(":team_name")
	pipelineName := r.FormValue(":pipeline_name")
	client := handler.clientFactory.Build(r)
	team := client.Team(teamName)

	_, pipelineFound, err := team.Pipeline(pipelineName)
	if err != nil {
		return err
	}

	if !pipelineFound {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}

	_, jobFound, err := team.Job(pipelineName, jobName)
	if err != nil {
		return err
	}

	if !jobFound {
		w.WriteHeader(http.StatusNotFound)
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

	templateData := TemplateData{
		TeamName:     teamName,
		PipelineName: pipelineName,
		JobName:      jobName,

		Since:       since,
		Until:       until,
		QueryGroups: nil,
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
