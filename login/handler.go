package login

import (
	"html/template"
	"net/http"

	"github.com/pivotal-golang/lager"
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
	handler.template.Execute(w, TemplateData{})
}
