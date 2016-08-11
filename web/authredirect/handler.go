package authredirect

import (
	"net/http"
	"net/url"

	"github.com/concourse/atc/web"
	"github.com/concourse/go-concourse/concourse"
	"github.com/tedsuo/rata"
)

type Handler struct {
	web.HTTPHandlerWithError
}

func (handler Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := handler.HTTPHandlerWithError.ServeHTTP(w, r)
	if err == concourse.ErrUnauthorized || err == concourse.ErrForbidden {
		teamName := r.FormValue(":team_name")
		var path string
		if teamName == "" {
			path, err = web.Routes.CreatePathForRoute(web.LogIn, rata.Params{})
		} else {
			path, err = web.Routes.CreatePathForRoute(web.TeamLogIn, rata.Params{
				"team_name": teamName,
			})
		}
		if err != nil {
			return
		}

		if redirectURL, ok := handler.redirectTargetFor(r); ok {
			path += "?" + url.Values{
				"redirect": {redirectURL},
			}.Encode()
		}

		http.Redirect(w, r, path, http.StatusFound)
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (handler Handler) redirectTargetFor(r *http.Request) (string, bool) {
	if r.Method == "GET" {
		reqURL, ok := r.Context().Value(requestURLKey).(string)
		return reqURL, ok
	}

	referer := r.Referer()
	if referer != "" {
		return referer, true
	}

	return "", false
}
