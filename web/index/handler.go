package index

import (
	"fmt"
	"html/template"
	"net/http"
	"sort"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/radar"
	"github.com/concourse/atc/web/routes"
)

type handler struct {
	logger lager.Logger

	// used for providing resource state
	radar *radar.Radar

	db       db.DB
	configDB db.ConfigDB

	template *template.Template
}

func NewHandler(logger lager.Logger, radar *radar.Radar, db db.DB, configDB db.ConfigDB, template *template.Template) http.Handler {
	return &handler{
		logger: logger,

		radar: radar,

		db:       db,
		configDB: configDB,

		template: template,
	}
}

type TemplateData struct {
	Jobs   []JobStatus
	Groups map[string]bool
	Nodes  []DotNode
	Edges  []DotEdge
}

type DotNode struct {
	ID    string   `json:"v"`
	Value DotValue `json:"value"`
}

type DotEdge struct {
	Source      string   `json:"v"`
	Destination string   `json:"w"`
	Value       DotValue `json:"value"`
}

type DotValue map[string]interface{}

type JobStatus struct {
	Job          atc.JobConfig
	CurrentBuild db.Build
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	config, err := handler.configDB.GetConfig()
	if err != nil {
		handler.logger.Error("failed-to-load-config", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	groups := map[string]bool{}
	for _, group := range config.Groups {
		groups[group.Name] = false
	}

	enabledGroups, found := r.URL.Query()["groups"]
	if !found && len(config.Groups) > 0 {
		enabledGroups = []string{config.Groups[0].Name}
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

	currentBuilds := map[string]db.Build{}

	for _, job := range config.Jobs {
		currentBuild, err := handler.db.GetCurrentBuild(job.Name)
		if err != nil {
			currentBuild.Status = db.StatusPending
		}

		currentBuilds[job.Name] = currentBuild

		data.Jobs = append(data.Jobs, JobStatus{
			Job:          job,
			CurrentBuild: currentBuild,
		})
	}

	jobGroups := map[string][]string{}
	resourceGroups := map[string][]string{}

	for _, group := range config.Groups {
		for _, name := range group.Jobs {
			jobGroups[name] = append(jobGroups[name], group.Name)
		}

		for _, name := range group.Resources {
			resourceGroups[name] = append(resourceGroups[name], group.Name)
		}
	}

	for _, resource := range config.Resources {
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
				"labelType": "html",
				"label":     fmt.Sprintf(`<h1 class="resource"><a href="%s">%s</a></h1>`, resourceURI, resource.Name),
				"class":     "resource " + status,
				"groups":    resourceGroups[resource.Name],
			},
		})
	}

	gateways := map[string]bool{}

	for _, job := range config.Jobs {
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
				"labelType": "html",
				"label":     fmt.Sprintf(`<h1 class="job"><a href="%s">%s</a>`, buildURI, job.Name),
				"class":     "job " + string(currentBuild.Status),
				"groups":    jobGroups[job.Name],
			},
		})

		edges := map[string]DotEdge{}

		for _, input := range job.Inputs {
			if len(input.Passed) > 0 {
				var nodeID string

				if len(input.Passed) > 1 {
					gatewayID := "gateway"

					sort.Strings(input.Passed)

					for _, j := range input.Passed {
						gatewayID += "-" + j
					}

					nodeID = gatewayID

					if _, found := gateways[gatewayID]; !found {
						data.Nodes = append(data.Nodes, DotNode{
							ID: gatewayID,
							Value: DotValue{
								"label":   "",
								"gateway": true,
								"class":   "gateway",
							},
						})
					}

					data.Edges = append(data.Edges, DotEdge{
						Source:      nodeID,
						Destination: jobID,
						Value: DotValue{
							"id":        "gateway-" + nodeID + "-to-" + jobID,
							"arrowhead": "status",
							"status":    "normal",
						},
					})
				} else {
					nodeID = jobID
				}

				for _, passed := range input.Passed {
					currentBuild := currentBuilds[passed]

					passedJob, found := config.Jobs.Lookup(passed)
					if !found {
						panic("unknown job: " + passed)
					}

					existingEdge, found := edges[input.Resource+"-"+passed]
					if found {
						if len(passedJob.Inputs) > 1 {
							existingEdge.Value["label"] = existingEdge.Value["label"].(string) + "\n" + input.Resource
						}
					} else {
						value := DotValue{
							"id":        "job-input-" + jobNode(passed) + "-to-" + nodeID,
							"status":    string(currentBuild.Status),
							"arrowhead": "status",
						}

						if len(passedJob.Inputs) > 1 {
							value["label"] = input.Resource
						}

						edges[input.Resource+"-"+passed] = DotEdge{
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
					Value: DotValue{
						"id":        "resource-input-" + resourceNode(input.Resource) + "-to-" + jobID,
						"arrowhead": "status",
						"status":    "normal",
					},
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
					"id":        "job-output-" + jobID + "-to-" + resourceNode(output.Resource),
					"arrowhead": "status",
					"status":    string(currentBuild.Status),
				},
			})
		}
	}

	err = handler.template.Execute(w, data)
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
