package index

import (
	"html/template"
	"net/http"

	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

type TemplateData struct{}

func NewHandler(
	logger lager.Logger,
	pipelineDBFactory db.PipelineDBFactory,
	pipelineHandler func(db.PipelineDB) http.Handler,
	template *template.Template,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Session("index")
		pipelineDB, found, err := pipelineDBFactory.BuildDefault()
		if err != nil {
			log.Error("failed-to-load-pipelinedb", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			err = template.Execute(w, TemplateData{})
			if err != nil {
				log.Fatal("failed-to-build-template", err, lager.Data{})
			}

			return
		}

		pipelineHandler(pipelineDB).ServeHTTP(w, r)
	})
}
