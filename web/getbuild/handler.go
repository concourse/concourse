package getbuild

import (
	"html/template"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/group"
	"github.com/pivotal-golang/lager"
)

type server struct {
	logger lager.Logger

	template *template.Template
}

func NewServer(logger lager.Logger, template *template.Template) *server {
	return &server{
		logger: logger,

		template: template,
	}
}

type TemplateData struct {
	GroupStates []group.State

	Job    atc.JobConfig
	Builds []db.Build

	Build        db.Build
	Inputs       []db.BuildInput
	Outputs      []db.BuildOutput
	PipelineName string
}

func (server *server) GetBuild(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobName := r.FormValue(":job")
		if len(jobName) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		buildName := r.FormValue(":build")
		if len(buildName) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		config, _, found, err := pipelineDB.GetConfig()
		if err != nil {
			server.logger.Error("failed-to-load-config", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		job, found := config.Jobs.Lookup(jobName)
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		log := server.logger.Session("get-build", lager.Data{
			"job":   job.Name,
			"build": buildName,
		})

		build, found, err := pipelineDB.GetJobBuild(jobName, buildName)
		if err != nil {
			log.Error("get-build-failed", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		inputs, outputs, err := pipelineDB.GetBuildResources(build.ID)
		if err != nil {
			log.Error("failed-to-get-build-resources", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		bs, err := pipelineDB.GetAllJobBuilds(jobName)
		if err != nil {
			log.Error("get-all-builds-failed", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		templateData := TemplateData{
			GroupStates: group.States(config.Groups, func(g atc.GroupConfig) bool {
				for _, groupJob := range g.Jobs {
					if groupJob == job.Name {
						return true
					}
				}

				return false
			}),

			Job:    job,
			Builds: bs,

			Build:        build,
			Inputs:       inputs,
			Outputs:      outputs,
			PipelineName: pipelineDB.GetPipelineName(),
		}

		err = server.template.Execute(w, templateData)
		if err != nil {
			log.Fatal("failed-to-build-template", err, lager.Data{
				"template-data": templateData,
			})
		}
	})
}
