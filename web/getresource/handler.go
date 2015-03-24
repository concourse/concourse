package getresource

import (
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
	Resource atc.ResourceConfig
	History  []*db.VersionHistory

	FailingToCheck bool
	CheckError     error

	GroupStates []group.State
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	config, _, err := handler.configDB.GetConfig()
	if err != nil {
		handler.logger.Error("failed-to-load-config", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resource, found := config.Resources.Lookup(r.FormValue(":resource"))
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	history, err := handler.db.GetResourceHistory(resource.Name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	templateData := TemplateData{
		Resource: resource,
		History:  history,

		GroupStates: group.States(config.Groups, func(g atc.GroupConfig) bool {
			for _, groupResource := range g.Resources {
				if groupResource == resource.Name {
					return true
				}
			}

			return false
		}),
	}

	checkErr, err := handler.db.GetResourceCheckError(resource.Name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	templateData.FailingToCheck = checkErr != nil

	if handler.validator.IsAuthenticated(r) {
		templateData.CheckError = checkErr
	}

	err = handler.template.Execute(w, templateData)
	if err != nil {
		log.Fatal("failed-to-task-template", err, lager.Data{
			"template-data": templateData,
		})
	}
}
