package basicauth

import (
	"html/template"
	"net/http"

	"github.com/concourse/atc/web"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
)

type getHandler struct {
	logger        lager.Logger
	template      *template.Template
	clientFactory web.ClientFactory
}

func NewGetBasicAuthHandler(
	logger lager.Logger,
	template *template.Template,
) http.Handler {
	return &getHandler{
		logger:   logger,
		template: template,
	}
}

type TemplateData struct {
	TeamName     string
	RedirectPath string
}

func (h *getHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	teamName := r.FormValue(":team_name")
	redirect := r.FormValue("redirect")
	if redirect == "" {
		indexPath, err := web.Routes.CreatePathForRoute(web.Index, rata.Params{})
		if err != nil {
			h.logger.Error("failed-to-generate-index-path", err)
		} else {
			redirect = indexPath
		}
	}

	err := h.template.Execute(w, TemplateData{
		TeamName:     teamName,
		RedirectPath: redirect,
	})
	if err != nil {
		h.logger.Fatal("failed-to-build-template", err, lager.Data{})
		w.WriteHeader(http.StatusInternalServerError)
	}
}
