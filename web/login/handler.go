package login

import (
	"html/template"
	"net/http"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/web/routes"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
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
	Redirect  string
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	redirect := r.FormValue("redirect")
	if redirect == "" {
		indexPath, err := routes.Routes.CreatePathForRoute(routes.Index, rata.Params{})
		if err != nil {
			handler.logger.Error("failed-to-generate-index-path", err)
		} else {
			redirect = indexPath
		}
	}

	err := handler.template.Execute(w, TemplateData{
		Providers: handler.providers,
		Redirect:  redirect,
	})
	if err != nil {
		handler.logger.Info("failed-to-generate-index-template", lager.Data{
			"error": err.Error(),
		})
	}
}
