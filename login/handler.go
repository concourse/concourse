package login

import (
	"html/template"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/web"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/rata"
)

type handler struct {
	logger        lager.Logger
	clientFactory web.ClientFactory
	template      *template.Template
}

func NewHandler(
	logger lager.Logger,
	clientFactory web.ClientFactory,
	template *template.Template,
) http.Handler {
	return &handler{
		logger:        logger,
		clientFactory: clientFactory,
		template:      template,
	}
}

type TemplateData struct {
	AuthMethods []atc.AuthMethod
	Redirect    string
}

func (handler *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	redirect := r.FormValue("redirect")
	if redirect == "" {
		indexPath, err := web.Routes.CreatePathForRoute(web.Index, rata.Params{})
		if err != nil {
			handler.logger.Error("failed-to-generate-index-path", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else {
			redirect = indexPath
		}
	}

	client := handler.clientFactory.Build(r)

	teamName := r.FormValue(":team_name")
	if teamName == "" {
		teamName = atc.DefaultTeamName
	}
	team := client.Team(teamName)
	authMethods, err := team.ListAuthMethods()
	if err != nil {
		handler.logger.Error("failed-to-list-auth-methods", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = handler.template.Execute(w, TemplateData{
		AuthMethods: authMethods,
		Redirect:    redirect,
	})
	if err != nil {
		handler.logger.Info("failed-to-generate-index-template", lager.Data{
			"error": err.Error(),
		})
	}
}
