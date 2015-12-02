package getjoblessbuild

import (
	"errors"
	"html/template"
	"net/http"
	"strconv"

	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

type handler struct {
	logger lager.Logger

	db       BuildDB
	configDB db.ConfigDB

	template    *template.Template
	oldTemplate *template.Template
}

//go:generate counterfeiter . BuildDB

type BuildDB interface {
	GetBuild(int) (db.Build, bool, error)
}

func NewHandler(logger lager.Logger, db BuildDB, configDB db.ConfigDB, template *template.Template, oldTemplate *template.Template) http.Handler {
	return &handler{
		logger: logger,

		db: db,

		configDB: configDB,

		template:    template,
		oldTemplate: oldTemplate,
	}
}

type TemplateData struct {
	Build db.Build
}

var ErrInvalidBuildID = errors.New("invalid build id")

func FetchTemplateData(buildID string, buildDB BuildDB, configDB db.ConfigDB) (TemplateData, error) {
	id, err := strconv.Atoi(buildID)
	if err != nil {
		return TemplateData{}, ErrInvalidBuildID
	}

	build, found, err := buildDB.GetBuild(id)
	if err != nil {
		return TemplateData{}, err
	}

	if !found {
		return TemplateData{}, nil
	}

	return TemplateData{
		Build: build,
	}, nil
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log := handler.logger.Session("jobless-build")
	templateData, err := FetchTemplateData(r.FormValue(":build_id"), handler.db, handler.configDB)
	if err != nil {
		log.Error("failed-to-build-template-data", err)
		http.Error(w, "failed to fetch builds", http.StatusInternalServerError)
		return
	}

	err = handler.template.Execute(w, templateData)
	if err != nil {
		log.Fatal("failed-to-build-template", err)
	}
}
