package getresource

import (
	"errors"
	"html/template"
	"log"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/group"
	"github.com/pivotal-golang/lager"
)

type server struct {
	logger lager.Logger

	validator auth.Validator

	template *template.Template
}

func NewServer(logger lager.Logger, template *template.Template, validator auth.Validator) *server {
	return &server{
		logger: logger,

		validator: validator,

		template: template,
	}
}

type TemplateData struct {
	Resource   atc.ResourceConfig
	DBResource db.SavedResource
	History    []*db.VersionHistory

	FailingToCheck bool
	CheckError     error

	GroupStates  []group.State
	PipelineName string
}

//go:generate counterfeiter . ResourcesDB

type ResourcesDB interface {
	GetPipelineName() string
	GetConfig() (atc.Config, db.ConfigVersion, error)
	GetResource(string) (db.SavedResource, error)
	GetResourceHistory(string) ([]*db.VersionHistory, error)
}

var ErrResourceConfigNotFound = errors.New("could not find resource")

func FetchTemplateData(resourceDB ResourcesDB, resourceName string) (TemplateData, error) {
	config, _, err := resourceDB.GetConfig()
	if err != nil {
		return TemplateData{}, err
	}

	configResource, found := config.Resources.Lookup(resourceName)
	if !found {
		return TemplateData{}, ErrResourceConfigNotFound
	}

	history, err := resourceDB.GetResourceHistory(configResource.Name)
	if err != nil {
		return TemplateData{}, err
	}

	resource, err := resourceDB.GetResource(configResource.Name)
	if err != nil {
		return TemplateData{}, err
	}

	templateData := TemplateData{
		Resource:     configResource,
		DBResource:   resource,
		History:      history,
		PipelineName: resourceDB.GetPipelineName(),
		GroupStates: group.States(config.Groups, func(g atc.GroupConfig) bool {
			for _, groupResource := range g.Resources {
				if groupResource == configResource.Name {
					return true
				}
			}

			return false
		}),
	}

	return templateData, nil
}

func (server *server) GetResource(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := r.FormValue(":resource")
		templateData, err := FetchTemplateData(pipelineDB, resourceName)

		switch err {
		case ErrResourceConfigNotFound:
			server.logger.Error("could-not-find-resource-in-config", ErrResourceConfigNotFound, lager.Data{
				"resource": resourceName,
			})
			w.WriteHeader(http.StatusNotFound)
			return
		case nil:
			break
		default:
			server.logger.Error("failed-to-build-template-data", err, lager.Data{
				"resource": resourceName,
			})
			http.Error(w, "failed to fetch resources", http.StatusInternalServerError)
			return
		}

		err = server.template.Execute(w, templateData)
		if err != nil {
			log.Fatal("failed-to-task-template", err, lager.Data{
				"template-data": templateData,
			})
		}
	})
}
