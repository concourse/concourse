package index

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/tedsuo/router"
	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/server/routes"
)

type handler struct {
	resources config.Resources
	jobs      config.Jobs
	db        db.DB
	template  *template.Template
}

func NewHandler(resources config.Resources, jobs config.Jobs, db db.DB, template *template.Template) http.Handler {
	return &handler{
		resources: resources,
		jobs:      jobs,
		db:        db,
		template:  template,
	}
}

type TemplateData struct {
	Jobs  []JobStatus
	Nodes []DotNode
	Edges []DotEdge
}

type DotNode struct {
	ID    string            `json:"id":`
	Value map[string]string `json:"value,omitempty"`
}

type DotEdge struct {
	Source      string            `json:"u"`
	Destination string            `json:"v"`
	Value       map[string]string `json:"value,omitempty"`
}

type JobStatus struct {
	Job          config.Job
	CurrentBuild builds.Build

	Nodes string
	Edges string
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data := TemplateData{}

	currentBuilds := map[string]builds.Build{}

	for _, job := range handler.jobs {
		currentBuild, err := handler.db.GetCurrentBuild(job.Name)
		if err != nil {
			currentBuild.Status = builds.StatusPending
		}

		currentBuilds[job.Name] = currentBuild

		data.Jobs = append(data.Jobs, JobStatus{
			Job:          job,
			CurrentBuild: currentBuild,
		})
	}

	for _, resource := range handler.resources {
		resourceID := resourceNode(resource.Name)

		data.Nodes = append(data.Nodes, DotNode{
			ID: resourceID,
			Value: map[string]string{
				"label": fmt.Sprintf(`<h1 class="resource">%s</a>`, resource.Name),
				"type":  "resource",
			},
		})
	}

	for _, job := range handler.jobs {
		jobID := jobNode(job.Name)
		currentBuild := currentBuilds[job.Name]

		buildURI, _ := routes.Routes.PathForHandler(routes.GetBuild, router.Params{
			"job":   job.Name,
			"build": fmt.Sprintf("%d", currentBuild.ID),
		})

		data.Nodes = append(data.Nodes, DotNode{
			ID: jobID,
			Value: map[string]string{
				"label":  fmt.Sprintf(`<h1 class="job"><a href="%s">%s</a>`, buildURI, job.Name),
				"status": string(currentBuild.Status),
				"type":   "job",
			},
		})

		for _, input := range job.Inputs {
			if len(input.Passed) > 0 {
				for _, passed := range input.Passed {
					currentBuild := currentBuilds[passed]

					data.Edges = append(data.Edges, DotEdge{
						Source:      jobNode(passed),
						Destination: jobID,
						Value: map[string]string{
							"status": string(currentBuild.Status),
						},
					})
				}
			} else {
				data.Edges = append(data.Edges, DotEdge{
					Source:      resourceNode(input.Resource),
					Destination: jobID,
				})
			}
		}

		for _, output := range job.Outputs {
			data.Edges = append(data.Edges, DotEdge{
				Source:      jobID,
				Destination: resourceNode(output.Resource),
				Value: map[string]string{
					"status": string(currentBuild.Status),
				},
			})
		}
	}

	err := handler.template.Execute(w, data)
	if err != nil {
		log.Println("failed to execute template:", err)
	}
}

func resourceNode(resource string) string {
	return "resource-" + resource
}

func jobNode(job string) string {
	return "job-" + job
}
