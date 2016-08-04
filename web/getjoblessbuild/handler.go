package getjoblessbuild

import (
	"html/template"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/web"
	"code.cloudfoundry.org/lager"
)

type Handler struct {
	logger        lager.Logger
	clientFactory web.ClientFactory
	template      *template.Template
}

func NewHandler(logger lager.Logger, clientFactory web.ClientFactory, template *template.Template) *Handler {
	return &Handler{
		logger:        logger,
		clientFactory: clientFactory,
		template:      template,
	}
}

type TemplateData struct {
	Build atc.Build
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	client := handler.clientFactory.Build(r)

	buildID := r.FormValue(":build_id")

	log := handler.logger.Session("one-off-build", lager.Data{
		"build-id": buildID,
	})

	build, found, err := client.Build(buildID)
	if err != nil {
		log.Error("failed-to-get-build", err)
		return err
	}

	if !found {
		log.Info("build-not-found")
		w.WriteHeader(http.StatusNotFound)
		return nil
	}

	err = handler.template.Execute(w, TemplateData{
		Build: build,
	})
	if err != nil {
		log.Fatal("failed-to-execute-template", err)
		return err
	}

	return nil
}
