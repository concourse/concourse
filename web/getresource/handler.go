package getresource

import (
	"errors"
	"html/template"
	"net/http"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/atc/web"
	"github.com/concourse/atc/web/group"
	"github.com/concourse/go-concourse/concourse"
	"code.cloudfoundry.org/lager"
)

type Handler struct {
	logger        lager.Logger
	clientFactory web.ClientFactory
	template      *template.Template
}

func NewHandler(logger lager.Logger, clientFactory web.ClientFactory, template *template.Template) *Handler {
	return &Handler{
		logger:        logger,
		clientFactory: clientFactory,
		template:      template,
	}
}

type VersionedResourceWithInputsAndOutputs struct {
	VersionedResource atc.VersionedResource
	InputsTo          map[string][]atc.Build
	OutputsOf         map[string][]atc.Build
}

type TemplateData struct {
	Resource atc.Resource
	Versions []VersionedResourceWithInputsAndOutputs

	GroupStates  []group.State
	PipelineName string
	TeamName     string

	Pagination concourse.Pagination
}

var ErrConfigNotFound = errors.New("could not find config")
var ErrResourceNotFound = errors.New("could not find resource")

func FetchTemplateData(
	pipelineName string,
	resourceName string,
	team concourse.Team,
	page concourse.Page,
) (TemplateData, error) {
	pipeline, pipelineFound, err := team.Pipeline(pipelineName)
	if err != nil {
		return TemplateData{}, err
	}

	if !pipelineFound {
		return TemplateData{}, ErrConfigNotFound
	}

	resource, resourceFound, err := team.Resource(pipelineName, resourceName)
	if err != nil {
		return TemplateData{}, err
	}

	if !resourceFound {
		return TemplateData{}, ErrResourceNotFound
	}

	versionedResources, pagination, resourceVersionsFound, err := team.ResourceVersions(pipelineName, resourceName, page)
	if err != nil {
		return TemplateData{}, err
	}

	if !resourceVersionsFound {
		return TemplateData{}, ErrResourceNotFound
	}

	versions := []VersionedResourceWithInputsAndOutputs{}
	for _, versionedResource := range versionedResources {
		inputs, _, err := team.BuildsWithVersionAsInput(pipelineName, resourceName, versionedResource.ID)
		if err != nil {
			return TemplateData{}, err
		}

		outputs, _, err := team.BuildsWithVersionAsOutput(pipelineName, resourceName, versionedResource.ID)
		if err != nil {
			return TemplateData{}, err
		}

		inputsTo := map[string][]atc.Build{}
		outputsOf := map[string][]atc.Build{}

		for _, input := range inputs {
			if _, ok := inputsTo[input.JobName]; !ok {
				inputsTo[input.JobName] = []atc.Build{}
			}

			inputsTo[input.JobName] = append(inputsTo[input.JobName], input)
		}

		for _, output := range outputs {
			if _, ok := outputsOf[output.JobName]; !ok {
				outputsOf[output.JobName] = []atc.Build{}
			}

			outputsOf[output.JobName] = append(outputsOf[output.JobName], output)
		}

		versions = append(versions, VersionedResourceWithInputsAndOutputs{
			VersionedResource: versionedResource,
			InputsTo:          inputsTo,
			OutputsOf:         outputsOf,
		})
	}

	return TemplateData{
		Resource:     resource,
		Versions:     versions,
		PipelineName: pipelineName,
		TeamName:     team.Name(),
		Pagination:   pagination,
		GroupStates: group.States(pipeline.Groups, func(g atc.GroupConfig) bool {
			for _, groupResource := range g.Resources {
				if groupResource == resource.Name {
					return true
				}
			}

			return false
		}),
	}, nil
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	session := handler.logger.Session("get-resource")

	teamName := r.FormValue(":team_name")
	pipelineName := r.FormValue(":pipeline_name")
	resourceName := r.FormValue(":resource")

	since, parseErr := strconv.Atoi(r.FormValue("since"))
	if parseErr != nil {
		since = 0
	}

	until, parseErr := strconv.Atoi(r.FormValue("until"))
	if parseErr != nil {
		until = 0
	}

	client := handler.clientFactory.Build(r)
	templateData, err := FetchTemplateData(
		pipelineName,
		resourceName,
		client.Team(teamName),
		concourse.Page{
			Since: since,
			Until: until,
			Limit: atc.PaginationWebLimit,
		},
	)

	switch err {
	case ErrResourceNotFound:
		session.Error("could-not-find-resource", ErrResourceNotFound, lager.Data{
			"resource": resourceName,
		})
		w.WriteHeader(http.StatusNotFound)
		return nil
	case ErrConfigNotFound:
		session.Error("could-not-find-config", ErrConfigNotFound, lager.Data{
			"pipeline": pipelineName,
		})
		w.WriteHeader(http.StatusNotFound)
		return nil
	case nil:
		break
	default:
		session.Error("failed-to-build-template-data", err, lager.Data{
			"resource": resourceName,
			"pipeline": pipelineName,
		})
		return err
	}

	err = handler.template.Execute(w, templateData)
	if err != nil {
		session.Fatal("failed-to-build-template", err, lager.Data{
			"template-data": templateData,
		})

		return err
	}

	return nil
}
