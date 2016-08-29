package mainredirect

import (
	"net/http"
	"strings"

	"github.com/tedsuo/rata"
)

type Handler struct {
	Routes rata.Routes
	Route  string
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

	path, err := h.Routes.CreatePathForRoute(h.Route, params)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, path, http.StatusMovedPermanently)
}
