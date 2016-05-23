package getbuild

import (
	"errors"
	"html/template"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/web"
	"github.com/concourse/atc/web/group"
	"github.com/concourse/go-concourse/concourse"
	"github.com/pivotal-golang/lager"
)

type TemplateData struct {
	GroupStates []group.State

	PipelineName string
	TeamName     string
	Job          atc.Job
	Build        atc.Build
}

type OldBuildTemplateData struct {
	TemplateData

	Builds []atc.Build
	Inputs []atc.PublicBuildInput
}

type Handler struct {
	logger           lager.Logger
	clientFactory    web.ClientFactory
	template         *template.Template
	oldBuildTemplate *template.Template
}

func NewHandler(
	logger lager.Logger,
	clientFactory web.ClientFactory,
	template *template.Template,
	oldBuildTemplate *template.Template,
) *Handler {
	return &Handler{
		logger:           logger,
		clientFactory:    clientFactory,
		template:         template,
		oldBuildTemplate: oldBuildTemplate,
	}
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	logger := handler.logger.Session("handler")

	client := handler.clientFactory.Build(r)

	teamName, pipelineName, jobName, buildName, err := getNames(r)
	if err != nil {
		logger.Error("failed-to-get-names", err)
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}

	team := client.Team(teamName)
	job, found, err := team.Job(pipelineName, jobName)
	if err != nil {
		logger.Error("failed-to-load-job", err)
		return err
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}

	log := logger.Session("get-build", lager.Data{
		"job":   job.Name,
		"build": buildName,
	})

	requestedBuild, found, err := team.JobBuild(pipelineName, jobName, buildName)
	if err != nil {
		log.Error("failed-to-get-build", err)
		return err
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}

	pipeline, _, err := team.Pipeline(pipelineName)
	if err != nil {
		log.Error("failed-to-get-pipeline", err)
		return err
	}

	templateData := TemplateData{
		GroupStates: group.States(pipeline.Groups, func(g atc.GroupConfig) bool {
			for _, groupJob := range g.Jobs {
				if groupJob == job.Name {
					return true
				}
			}

			return false
		}),

		Job: job,

		Build:        requestedBuild,
		PipelineName: pipelineName,
		TeamName:     teamName,
	}

	buildPlan, found, err := client.BuildPlan(requestedBuild.ID)
	if err != nil {
		log.Error("failed-to-get-build-plan", err)
		return err
	}

	if buildPlan.Schema == "exec.v2" || !found {
		// either it's definitely a new build, or it hasn't started yet (and thus
		// must be new), so render with the new UI
		err = handler.template.Execute(w, templateData)
		if err != nil {
			log.Fatal("failed-to-build-template", err, lager.Data{
				"template-data": templateData,
			})

			return err
		}
	} else {
		buildInputsOutputs, _, err := client.BuildResources(requestedBuild.ID)
		if err != nil {
			log.Error("failed-to-get-build-resources", err)
			return err
		}

		builds, err := getAllJobBuilds(team, pipelineName, jobName)
		if err != nil {
			log.Error("get-all-builds-failed", err)
			return err
		}

		oldBuildTemplateData := OldBuildTemplateData{
			TemplateData: templateData,
			Builds:       builds,
			Inputs:       buildInputsOutputs.Inputs,
		}

		err = handler.oldBuildTemplate.Execute(w, oldBuildTemplateData)
		if err != nil {
			log.Fatal("failed-to-build-template", err, lager.Data{
				"template-data": oldBuildTemplateData,
			})
			return err
		}
	}

	return nil
}

func getAllJobBuilds(team concourse.Team, pipelineName string, jobName string) ([]atc.Build, error) {
	builds := []atc.Build{}
	page := &concourse.Page{}

	for page != nil {
		bs, pagination, _, err := team.JobBuilds(pipelineName, jobName, *page)
		if err != nil {
			return nil, err
		}

		builds = append(builds, bs...)
		page = pagination.Next
	}

	return builds, nil
}

func getNames(r *http.Request) (string, string, string, string, error) {
	pipelineName := r.FormValue(":pipeline_name")
	teamName := r.FormValue(":team_name")
	jobName := r.FormValue(":job")
	buildName := r.FormValue(":build")

	if len(pipelineName) == 0 || len(jobName) == 0 || len(buildName) == 0 || len(teamName) == 0 {
		return "", "", "", "", errors.New("Missing required parameters")
	}

	return teamName, pipelineName, jobName, buildName, nil
}
