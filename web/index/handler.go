package index

import (
	"html/template"
	"net/http"
	"net/url"

	"github.com/concourse/atc/web"
	"github.com/pivotal-golang/lager"
)

type TemplateData struct{}

func NewHandler(
	logger lager.Logger,
	clientFactory web.ClientFactory,
	pipelineHandler http.Handler,
	noBuildsTemplate *template.Template,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Session("index")

		client := clientFactory.Build(r)

		pipelines, err := client.ListPipelines()
		if err != nil {
			log.Error("failed-to-load-pipelinedb", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if len(pipelines) == 0 {
			err = noBuildsTemplate.Execute(w, TemplateData{})
			if err != nil {
				log.Fatal("failed-to-build-template", err, lager.Data{})
			}

			return
		}

		if r.Form == nil {
			r.Form = url.Values{}
		}

		r.Form[":pipeline"] = []string{pipelines[0].Name}

		pipelineHandler.ServeHTTP(w, r)
	})
}
