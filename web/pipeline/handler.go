package pipeline

import (
	"html/template"
	"net/http"

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

	QueryGroups []string
	Elm         bool
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	pipelineName := r.FormValue(":pipeline")
	teamName := r.FormValue(":team_name")

	_, isElm := r.URL.Query()["elm"]
	queryGroups, found := r.URL.Query()["groups"]
	if !found {
		queryGroups = []string{}
	}

	data := TemplateData{
		PipelineName: pipelineName,
		TeamName:     teamName,
		QueryGroups:  queryGroups,
		Elm:          isElm,
	}

	log := handler.logger.Session("index")

	err := handler.template.Execute(w, data)
	if err != nil {
		log.Fatal("failed-to-build-template", err, lager.Data{
			"template-data": data,
		})
		return err
	}

	return nil
}
