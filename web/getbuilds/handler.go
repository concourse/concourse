package getbuilds

import (
	"html/template"
	"net/http"

	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

type handler struct {
	logger lager.Logger

	db       BuildsDB
	configDB db.ConfigDB

	template *template.Template
}

//go:generate counterfeiter . BuildsDB

type BuildsDB interface {
	GetAllBuilds() ([]db.Build, error)
}

func NewHandler(logger lager.Logger, db BuildsDB, configDB db.ConfigDB, template *template.Template) http.Handler {
	return &handler{
		logger: logger,

		db: db,

		configDB: configDB,

		template: template,
	}
}

type TemplateData struct {
	Builds []PresentedBuild
}

func FetchTemplateData(buildDB BuildsDB, configDB db.ConfigDB) (TemplateData, error) {
	builds, err := buildDB.GetAllBuilds()
	if err != nil {
		return TemplateData{}, err
	}

	return TemplateData{
		Builds: PresentBuilds(builds),
	}, nil
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log := handler.logger.Session("builds")

	templateData, err := FetchTemplateData(handler.db, handler.configDB)
	if err != nil {
		log.Error("failed-to-build-template-data", err)
		http.Error(w, "failed to fetch builds", http.StatusInternalServerError)
		return
	}

	err = handler.template.Execute(w, templateData)
	if err != nil {
		log.Fatal("failed-to-task-template", err)
	}
}
