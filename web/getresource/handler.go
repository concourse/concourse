package getresource

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
	PipelineName string
	TeamName     string
	ResourceName string

	Since       int
	Until       int
	QueryGroups []string
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	session := handler.logger.Session("get-resource")

	teamName := r.FormValue(":team_name")
	pipelineName := r.FormValue(":pipeline_name")
	resourceName := r.FormValue(":resource")

	since, parseErr := strconv.Atoi(r.FormValue("since"))
	if parseErr != nil {
		since = 0
	}

	until, parseErr := strconv.Atoi(r.FormValue("until"))
	if parseErr != nil {
		until = 0
	}

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

	_, resourceFound, err := team.Resource(pipelineName, resourceName)
	if err != nil {
		return err
	}

	if !resourceFound {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}

	templateData := TemplateData{
		TeamName:     teamName,
		PipelineName: pipelineName,
		ResourceName: resourceName,
		Since:        since,
		Until:        until,
		QueryGroups:  nil,
	}

	err = handler.template.Execute(w, templateData)
	if err != nil {
		session.Fatal("failed-to-build-template", err, lager.Data{
			"template-data": templateData,
		})

		return err
	}

	return nil
}
