package getresource

import (
	"html/template"
	"log"
	"net/http"

	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

type handler struct {
	logger lager.Logger

	resources config.Resources
	db        db.DB
	template  *template.Template
}

func NewHandler(logger lager.Logger, resources config.Resources, db db.DB, template *template.Template) http.Handler {
	return &handler{
		logger: logger,

		resources: resources,
		db:        db,
		template:  template,
	}
}

type TemplateData struct {
	History []db.VersionHistory
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resource, found := handler.resources.Lookup(r.FormValue(":resource"))
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	history, err := handler.db.GetResourceHistory(resource.Name)
	if err != nil {
		panic(err)
	}

	templateData := TemplateData{
		History: history,
	}

	err = handler.template.Execute(w, templateData)
	if err != nil {
		log.Fatal("failed-to-execute-template", err, lager.Data{
			"template-data": templateData,
		})
	}
}
