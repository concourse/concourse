package login

import (
	"html/template"
	"net/http"

	"github.com/concourse/atc/auth"
	"github.com/pivotal-golang/lager"
)

type handler struct {
	logger    lager.Logger
	providers auth.Providers
	template  *template.Template
}

func NewHandler(
	logger lager.Logger,
	providers auth.Providers,
	template *template.Template,
) http.Handler {
	return &handler{
		logger:    logger,
		providers: providers,
		template:  template,
	}
}

type TemplateData struct {
	Providers auth.Providers
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler.template.Execute(w, TemplateData{
		Providers: handler.providers,
	})
}
