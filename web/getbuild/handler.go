package getbuild

import (
	"html/template"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/web"
)

type TemplateData struct {
	QueryGroups []string
}

type Handler struct {
	logger        lager.Logger
	clientFactory web.ClientFactory
	template      *template.Template
}

func NewHandler(
	logger lager.Logger,
	clientFactory web.ClientFactory,
	template *template.Template,
) *Handler {
	return &Handler{
		logger:        logger,
		clientFactory: clientFactory,
		template:      template,
	}
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	logger := handler.logger.Session("handler")

	teamName := r.FormValue(":team_name")
	pipelineName := r.FormValue(":pipeline_name")
	jobName := r.FormValue(":job")
	buildName := r.FormValue(":build")

	client := handler.clientFactory.Build(r)
	team := client.Team(teamName)

	log := logger.Session("get-build", lager.Data{
		"pipeline": pipelineName,
		"job":      jobName,
		"build":    buildName,
	})

	_, found, err := team.JobBuild(pipelineName, jobName, buildName)
	if err != nil {
		log.Error("failed-to-get-build", err)
		return err
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}

	templateData := TemplateData{
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
