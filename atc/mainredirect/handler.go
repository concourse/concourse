package mainredirect

import (
	"net/http"
	"strings"

	"github.com/concourse/concourse/atc/api/present"
)

type Handler struct {
	Route       string
	PathBuilder present.PathBuilder
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	params := map[string]string{"team_name": "main"}

	for k, vs := range r.URL.Query() {
		if strings.HasPrefix(k, ":") {
			params[k[1:]] = vs[0]
		}
	}

	path, err := h.PathBuilder.CreatePathForRoute(h.Route, params)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, path, http.StatusMovedPermanently)
}
