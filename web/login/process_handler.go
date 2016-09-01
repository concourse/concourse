package login

import (
	"fmt"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/web"
	"github.com/tedsuo/rata"
)

const CookieName = "ATC-Authorization"

type processHandler struct {
	logger        lager.Logger
	clientFactory web.ClientFactory
	expire        time.Duration
}

func NewProcessBasicAuthHandler(
	logger lager.Logger,
	clientFactory web.ClientFactory,
	expire time.Duration,
) http.Handler {
	return &processHandler{
		logger:        logger,
		clientFactory: clientFactory,
		expire:        expire,
	}
}

func (h *processHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	teamName := r.FormValue(":team_name")
	username := r.FormValue("username")
	password := r.FormValue("password")
	redirect := r.FormValue("redirect")

	if redirect == "" {
		indexPath, err := web.Routes.CreatePathForRoute(web.Index, rata.Params{})
		if err != nil {
			h.logger.Error("failed-to-generate-index-path", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		redirect = indexPath
	}

	r.SetBasicAuth(username, password)
	client := h.clientFactory.Build(r)
	team := client.Team(teamName)

	token, err := team.AuthToken()
	if err != nil {
		h.logger.Error("failed-to-get-token", err, lager.Data{})
		w.WriteHeader(http.StatusInternalServerError)
	}

	http.SetCookie(w, &http.Cookie{
		Name:    CookieName,
		Value:   fmt.Sprintf("%s %s", token.Type, token.Value),
		Path:    "/",
		Expires: time.Now().Add(h.expire),
	})

	http.Redirect(w, r, redirect, http.StatusFound)
}
