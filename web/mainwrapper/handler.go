package mainwrapper

import (
	"net/http"
	"strings"

	"github.com/concourse/atc/web"
	"github.com/tedsuo/rata"
)

type Handler struct {
	Route string

	http.Handler
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	params := rata.Params{
		"team_name": "main",
	}

	for k, vs := range r.URL.Query() {
		if strings.HasPrefix(k, ":") {
			params[k[1:]] = vs[0]
		}
	}

	path, err := web.Routes.CreatePathForRoute(h.Route, params)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, path, http.StatusMovedPermanently)
}
