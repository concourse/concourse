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

type handler struct {
	logger lager.Logger

	db       db.DB
	configDB db.ConfigDB

	validator auth.Validator

	template *template.Template
}

func NewHandler(logger lager.Logger, db db.DB, configDB db.ConfigDB, template *template.Template, validator auth.Validator) http.Handler {
	return &handler{
		logger: logger,

		db:       db,
		configDB: configDB,

		validator: validator,

		template: template,
	}
}

type TemplateData struct {
	Resource   atc.ResourceConfig
	DBResource db.Resource
	History    []*db.VersionHistory

	FailingToCheck bool
	CheckError     error

	GroupStates []group.State
}

//go:generate counterfeiter . ResourcesDB

type ResourcesDB interface {
	GetResource(string) (db.Resource, error)
	GetResourceHistory(string) ([]*db.VersionHistory, error)
}

var ErrResourceConfigNotFound = errors.New("could not find resource")

func FetchTemplateData(resourceDB ResourcesDB, configDB db.ConfigDB, resourceName string) (TemplateData, error) {
	config, _, err := configDB.GetConfig(atc.DefaultPipelineName)
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
		Resource:   configResource,
		DBResource: resource,
		History:    history,

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

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resourceName := r.FormValue(":resource")
	templateData, err := FetchTemplateData(handler.db, handler.configDB, resourceName)

	switch err {
	case ErrResourceConfigNotFound:
		handler.logger.Error("could-not-find-resource-in-config", ErrResourceConfigNotFound, lager.Data{
			"resource": resourceName,
		})
		w.WriteHeader(http.StatusNotFound)
		return
	case nil:
		break
	default:
		handler.logger.Error("failed-to-build-template-data", err, lager.Data{
			"resource": resourceName,
		})
		http.Error(w, "failed to fetch resources", http.StatusInternalServerError)
		return
	}

	err = handler.template.Execute(w, templateData)
	if err != nil {
		log.Fatal("failed-to-task-template", err, lager.Data{
			"template-data": templateData,
		})
	}
}
