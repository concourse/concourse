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

	groups    config.Groups
	resources config.Resources
	jobs      config.Jobs
	db        db.DB
	template  *template.Template
}

func NewHandler(logger lager.Logger, radar *radar.Radar, groups config.Groups, resources config.Resources, jobs config.Jobs, db db.DB, template *template.Template) http.Handler {
	return &handler{
		logger: logger,

		radar: radar,

		groups:    groups,
		resources: resources,
		jobs:      jobs,
		db:        db,
		template:  template,
	}
}

type TemplateData struct {
	Jobs   []JobStatus
	Groups map[string]bool
	Nodes  []DotNode
	Edges  []DotEdge
}

type DotNode struct {
	ID    string   `json:"id"`
	Value DotValue `json:"value,omitempty"`
}

type DotEdge struct {
	Source      string   `json:"u"`
	Destination string   `json:"v"`
	Value       DotValue `json:"value,omitempty"`
}

type DotValue map[string]interface{}

type JobStatus struct {
	Job          config.Job
	CurrentBuild builds.Build
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	groups := map[string]bool{}
	for _, group := range handler.groups {
		groups[group.Name] = false
	}

	enabledGroups, found := r.URL.Query()["groups"]
	if !found && len(handler.groups) > 0 {
		enabledGroups = []string{handler.groups[0].Name}
	}

	for _, name := range enabledGroups {
		groups[name] = true
	}

	data := TemplateData{
		Groups: groups,
		Nodes:  []DotNode{},
		Edges:  []DotEdge{},
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

	jobGroups := map[string][]string{}
	resourceGroups := map[string][]string{}

	for _, group := range handler.groups {
		for _, name := range group.Jobs {
			jobGroups[name] = append(jobGroups[name], group.Name)
		}

		for _, name := range group.Resources {
			resourceGroups[name] = append(resourceGroups[name], group.Name)
		}
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
			Value: DotValue{
				"label":  fmt.Sprintf(`<h1 class="resource"><a href="%s">%s</a></h1>`, resourceURI, resource.Name),
				"type":   "resource",
				"status": status,
				"groups": resourceGroups[resource.Name],
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
			Value: DotValue{
				"label":  fmt.Sprintf(`<h1 class="job"><a href="%s">%s</a>`, buildURI, job.Name),
				"status": string(currentBuild.Status),
				"type":   "job",
				"groups": jobGroups[job.Name],
			},
		})

		edges := map[string]DotEdge{}

		for _, input := range job.Inputs {
			if input.DontCheck {
				continue
			}

			if len(input.Passed) > 0 {
				var nodeID string

				if len(input.Passed) > 1 {
					nodeID = jobID + "-input-" + input.Resource

					data.Nodes = append(data.Nodes, DotNode{
						ID: nodeID,
						Value: DotValue{
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

					existingEdge, found := edges[passed]
					if found {
						if len(passedJob.Inputs) > 1 {
							existingEdge.Value["label"] = existingEdge.Value["label"].(string) + "\n" + input.Resource
						}
					} else {
						value := DotValue{
							"status": string(currentBuild.Status),
						}

						if len(passedJob.Inputs) > 1 {
							value["label"] = input.Resource
						}

						edges[passed] = DotEdge{
							Source:      jobNode(passed),
							Destination: nodeID,
							Value:       value,
						}
					}
				}
			} else {
				data.Edges = append(data.Edges, DotEdge{
					Source:      resourceNode(input.Resource),
					Destination: jobID,
				})
			}
		}

		for _, edge := range edges {
			data.Edges = append(data.Edges, edge)
		}

		for _, output := range job.Outputs {
			data.Edges = append(data.Edges, DotEdge{
				Source:      jobID,
				Destination: resourceNode(output.Resource),
				Value: DotValue{
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
