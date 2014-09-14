package index

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/radar"
	"github.com/concourse/atc/web/routes"
)

type handler struct {
	logger lager.Logger

	// used for providing resource state
	radar *radar.Radar

	resources config.Resources
	jobs      config.Jobs
	db        db.DB
	template  *template.Template
}

func NewHandler(logger lager.Logger, radar *radar.Radar, resources config.Resources, jobs config.Jobs, db db.DB, template *template.Template) http.Handler {
	return &handler{
		logger: logger,

		radar: radar,

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
	ID    string            `json:"id"`
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
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data := TemplateData{
		Nodes: []DotNode{},
		Edges: []DotEdge{},
	}

	log := handler.logger.Session("index")

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

		resourceURI, _ := routes.Routes.CreatePathForRoute(routes.GetResource, rata.Params{
			"resource": resource.Name,
		})

		failing, checking := handler.radar.ResourceStatus(resource.Name)

		var status string
		if failing {
			status = "failing"
		} else {
			status = "ok"
		}

		if checking {
			status += " checking"
		}

		data.Nodes = append(data.Nodes, DotNode{
			ID: resourceID,
			Value: map[string]string{
				"label":  fmt.Sprintf(`<h1 class="resource"><a href="%s">%s</a></h1>`, resourceURI, resource.Name),
				"type":   "resource",
				"status": status,
			},
		})
	}

	for _, job := range handler.jobs {
		jobID := jobNode(job.Name)
		currentBuild := currentBuilds[job.Name]

		var buildURI string
		var err error

		if currentBuild.Name != "" {
			buildURI, err = routes.Routes.CreatePathForRoute(routes.GetBuild, rata.Params{
				"job":   job.Name,
				"build": currentBuild.Name,
			})
		} else {
			buildURI, err = routes.Routes.CreatePathForRoute(routes.GetJob, rata.Params{
				"job": job.Name,
			})
		}

		if err != nil {
			log.Error("failed-to-create-route", err)
		}

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
				var nodeID string

				if len(input.Passed) > 1 {
					nodeID = jobID + "-input-" + input.Resource

					data.Nodes = append(data.Nodes, DotNode{
						ID: nodeID,
						Value: map[string]string{
							"useDef": "gateway",
						},
					})

					data.Edges = append(data.Edges, DotEdge{
						Source:      nodeID,
						Destination: jobID,
					})
				} else {
					nodeID = jobID
				}

				for _, passed := range input.Passed {
					currentBuild := currentBuilds[passed]

					passedJob, found := handler.jobs.Lookup(passed)
					if !found {
						panic("unknown job: " + passed)
					}

					value := map[string]string{
						"status": string(currentBuild.Status),
					}

					if len(passedJob.Inputs) > 1 {
						value["label"] = input.Resource
					}

					data.Edges = append(data.Edges, DotEdge{
						Source:      jobNode(passed),
						Destination: nodeID,
						Value:       value,
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
		log.Fatal("failed-to-execute-template", err, lager.Data{
			"template-data": data,
		})
	}
}

func resourceNode(resource string) string {
	return "resource-" + resource
}

func jobNode(job string) string {
	return "job-" + job
}
