package basicauth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/concourse/atc/web"
	"code.cloudfoundry.org/lager"
)

const CookieName = "ATC-Authorization"
const CookieAge = 24 * time.Hour

type processHandler struct {
	logger        lager.Logger
	clientFactory web.ClientFactory
}

func NewProcessBasicAuthHandler(
	logger lager.Logger,
	clientFactory web.ClientFactory,
) http.Handler {
	return &processHandler{
		logger:        logger,
		clientFactory: clientFactory,
	}
}

func (h *processHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	teamName := r.FormValue(":team_name")
	redirect := r.FormValue("redirect")
	username := r.FormValue("username")
	password := r.FormValue("password")

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
		Expires: time.Now().Add(CookieAge),
	})

	http.Redirect(w, r, redirect, http.StatusFound)
}
