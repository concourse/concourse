package login

import (
	"html/template"
	"net/http"

	"code.cloudfoundry.org/lager"
)

type handler struct {
	logger   lager.Logger
	template *template.Template
}

func NewHandler(
	logger lager.Logger,
	template *template.Template,
) http.Handler {
	return &handler{
		logger:   logger,
		template: template,
	}
}

type TemplateData struct{}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := handler.template.Execute(w, TemplateData{})
	if err != nil {
		handler.logger.Info("failed-to-generate-login-template", lager.Data{
			"error": err.Error(),
		})
	}
}
