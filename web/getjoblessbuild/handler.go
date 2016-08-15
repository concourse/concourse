package getjoblessbuild

import (
	"html/template"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/web"
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
	// no-op so the template can be reused until we elm-ify nav
	PipelineName string
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	err := handler.template.Execute(w, TemplateData{})
	if err != nil {
		handler.logger.Fatal("failed-to-execute-jobless-build-template", err)
		return err
	}

	return nil
}
