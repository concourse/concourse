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
	Job          atc.Job
	Build        atc.Build
}

type OldBuildTemplateData struct {
	TemplateData

	Builds []atc.Build
	Inputs []atc.PublicBuildInput
}

func getNames(r *http.Request) (string, string, string, error) {
	pipelineName := r.FormValue(":pipeline_name")
	jobName := r.FormValue(":job")
	buildName := r.FormValue(":build")

	if len(pipelineName) == 0 || len(jobName) == 0 || len(buildName) == 0 {
		return "", "", "", errors.New("Missing required parameters")
	}

	return pipelineName, jobName, buildName, nil
}

func NewHandler(
	logger lager.Logger,
	clientFactory web.ClientFactory,
	template *template.Template,
	oldBuildTemplate *template.Template,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client := clientFactory.Build(r)

		pipelineName, jobName, buildName, err := getNames(r)
		if err != nil {
			logger.Error("failed-to-get-names", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		job, found, err := client.Job(pipelineName, jobName)
		if err != nil {
			logger.Error("failed-to-load-job", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		log := logger.Session("get-build", lager.Data{
			"job":   job.Name,
			"build": buildName,
		})

		requestedBuild, found, err := client.JobBuild(pipelineName, jobName, buildName)
		if err != nil {
			log.Error("get-build-failed", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		pipeline, _, err := client.Pipeline(pipelineName)
		if err != nil {
			log.Error("get-pipeline-failed", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
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
		}

		buildPlan, found, err := client.BuildPlan(requestedBuild.ID)
		if err != nil {
			log.Error("get-build-plan-failed", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if buildPlan.Schema == "exec.v2" || !found {
			// either it's definitely a new build, or it hasn't started yet (and thus
			// must be new), so render with the new UI
			err = template.Execute(w, templateData)
			if err != nil {
				log.Fatal("failed-to-build-template", err, lager.Data{
					"template-data": templateData,
				})
			}
		} else {
			buildInputsOutputs, _, err := client.BuildResources(requestedBuild.ID)
			if err != nil {
				log.Error("failed-to-get-build-resources", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			builds, err := getAllJobBuilds(client, pipelineName, jobName)
			if err != nil {
				log.Error("get-all-builds-failed", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			oldBuildTemplateData := OldBuildTemplateData{
				TemplateData: templateData,
				Builds:       builds,
				Inputs:       buildInputsOutputs.Inputs,
			}

			err = oldBuildTemplate.Execute(w, oldBuildTemplateData)
			if err != nil {
				log.Fatal("failed-to-build-template", err, lager.Data{
					"template-data": oldBuildTemplateData,
				})
			}
		}
	})
}

func getAllJobBuilds(client concourse.Client, pipelineName string, jobName string) ([]atc.Build, error) {
	builds := []atc.Build{}
	page := &concourse.Page{}

	for page != nil {
		bs, pagination, _, err := client.JobBuilds(pipelineName, jobName, *page)
		if err != nil {
			return nil, err
		}

		builds = append(builds, bs...)
		page = pagination.Next
	}

	return builds, nil
}
