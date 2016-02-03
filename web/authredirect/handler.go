package authredirect

import (
	"net/http"
	"net/url"

	"github.com/concourse/atc/web"
	"github.com/concourse/go-concourse/concourse"
)

type ErrHandler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request) error
}

type Handler struct {
	ErrHandler
}

func (handler Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := handler.ErrHandler.ServeHTTP(w, r)
	if err == concourse.ErrUnauthorized {
		path, err := web.Routes.CreatePathForRoute(web.LogIn, nil)
		if err != nil {
			return
		}

		path += "?" + url.Values{
			"redirect": []string{r.URL.String()},
		}.Encode()

		http.Redirect(w, r, path, http.StatusFound)
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
