package web

import (
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/tedsuo/rata"

	"code.cloudfoundry.org/lager"
)

const CookieName = "ATC-Authorization"

type templateData struct{}

type handler struct {
	logger   lager.Logger
	template *template.Template
}

func NewHandler(logger lager.Logger) (http.Handler, error) {
	tfuncs := &templateFuncs{
		assetIDs: map[string]string{},
	}

	funcs := template.FuncMap{
		"asset": tfuncs.asset,
	}

	src, err := Asset("index.html")
	if err != nil {
		return nil, err
	}

	t, err := template.New("index").Funcs(funcs).Parse(string(src))
	if err != nil {
		return nil, err
	}

	return &handler{
		logger:   logger,
		template: t,
	}, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		h.serveLoginPost(w, r)
		return
	}

	log := h.logger.Session("index")

	err := h.template.Execute(w, templateData{})
	if err != nil {
		log.Fatal("failed-to-build-template", err, lager.Data{})
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (h *handler) serveLoginPost(w http.ResponseWriter, r *http.Request) {
	// teamName := r.FormValue(":team_name")
	username := r.FormValue("username")
	password := r.FormValue("password")
	redirect := r.FormValue("redirect")

	if redirect == "" {
		indexPath, err := Routes.CreatePathForRoute(Index, rata.Params{})
		if err != nil {
			h.logger.Error("failed-to-generate-index-path", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		redirect = indexPath
	}

	r.SetBasicAuth(username, password)
	client := NewClientFactory("http://127.0.0.1:8080/", true).Build(r)
	team := client.Team("main")

	token, err := team.AuthToken()
	if err != nil {
		h.logger.Error("failed-to-get-token", err, lager.Data{})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:    CookieName,
		Value:   fmt.Sprintf("%s %s", token.Type, token.Value),
		Path:    "/",
		Expires: time.Now().Add(12 * time.Hour),
	})

	http.Redirect(w, r, redirect, http.StatusFound)
}
