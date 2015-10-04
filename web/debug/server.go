package debug

import (
	"html/template"
	"net/http"

	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/db"
)

type server struct {
	logger lager.Logger

	db DebugDB

	template *template.Template
}

//go:generate counterfeiter . DebugDB

type DebugDB interface {
	FindContainerInfosByIdentifier(db.ContainerIdentifier) ([]db.ContainerInfo, error)
	Workers() ([]db.WorkerInfo, error)
}

func NewServer(logger lager.Logger, db DebugDB, template *template.Template) http.Handler {
	return &server{
		logger:   logger,
		db:       db,
		template: template,
	}
}

type WorkMap map[string][]db.ContainerInfo

type TemplateData struct {
	WorkMap WorkMap
}

func (server *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	log := server.logger.Session("builds")

	containers, err := server.db.FindContainerInfosByIdentifier(db.ContainerIdentifier{})
	if err != nil {
		log.Error("fetching-container-info-failed", err)
		http.Error(w, err.Error(), 500)
		return
	}

	workMap := WorkMap{}

	for _, container := range containers {
		if _, found := workMap[container.WorkerName]; !found {
			workMap[container.WorkerName] = []db.ContainerInfo{}
		}

		workMap[container.WorkerName] = append(
			workMap[container.WorkerName],
			container,
		)
	}

	err = server.template.Execute(w, TemplateData{
		WorkMap: workMap,
	})
	if err != nil {
		log.Fatal("failed-to-build-template", err)
	}
}
