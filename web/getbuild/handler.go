package getbuild

import (
	"html/template"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/web"
	"github.com/concourse/atc/web/group"
	"github.com/pivotal-golang/lager"
)

type TemplateData struct {
	GroupStates []group.State

	PipelineName string
	JobName      string

	Build atc.Build
}

type OldBuildTemplateData struct {
	TemplateData

	Builds []atc.Build
	Inputs []atc.PublicBuildInput
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

	client := handler.clientFactory.Build(r)

	pipelineName := r.FormValue(":pipeline_name")
	jobName := r.FormValue(":job")
	buildName := r.FormValue(":build")

	log := logger.Session("get-build", lager.Data{
		"pipeline": pipelineName,
		"job":      jobName,
		"build":    buildName,
	})

	requestedBuild, found, err := client.JobBuild(pipelineName, jobName, buildName)
	if err != nil {
		log.Error("failed-to-get-build", err)
		return err
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}

	pipeline, found, err := client.Pipeline(pipelineName)
	if err != nil {
		log.Error("failed-to-get-pipeline", err)
		return err
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}

	templateData := TemplateData{
		GroupStates: group.States(pipeline.Groups, func(g atc.GroupConfig) bool {
			for _, groupJob := range g.Jobs {
				if groupJob == jobName {
					return true
				}
			}

			return false
		}),

		PipelineName: pipelineName,
		JobName:      jobName,

		Build: requestedBuild,
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
