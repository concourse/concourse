package getresource

import (
	"html/template"
	"log"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

type handler struct {
	logger lager.Logger

	db       db.DB
	configDB db.ConfigDB

	template *template.Template
}

func NewHandler(logger lager.Logger, db db.DB, configDB db.ConfigDB, template *template.Template) http.Handler {
	return &handler{
		logger: logger,

		db:       db,
		configDB: configDB,

		template: template,
	}
}

type TemplateData struct {
	Resource atc.ResourceConfig
	History  []*db.VersionHistory
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	config, err := handler.configDB.GetConfig()
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
		panic(err)
	}

	templateData := TemplateData{
		Resource: resource,
		History:  history,
	}

	err = handler.template.Execute(w, templateData)
	if err != nil {
		log.Fatal("failed-to-execute-template", err, lager.Data{
			"template-data": templateData,
		})
	}
}
