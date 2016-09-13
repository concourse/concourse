package getresource

import (
	"html/template"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/lager"
)

type Handler struct {
	logger   lager.Logger
	template *template.Template
}

func NewHandler(logger lager.Logger, template *template.Template) *Handler {
	return &Handler{
		logger:   logger,
		template: template,
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

	templateData := TemplateData{
		TeamName:     teamName,
		PipelineName: pipelineName,
		ResourceName: resourceName,
		Since:        since,
		Until:        until,
		QueryGroups:  nil,
	}

	err := handler.template.Execute(w, templateData)
	if err != nil {
		session.Fatal("failed-to-build-template", err, lager.Data{
			"template-data": templateData,
		})

		return err
	}

	return nil
}
