package auth

import "net/http"

type RedirectRejector struct {
	Location string
}

func (rejector RedirectRejector) Unauthorized(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, rejector.Location, http.StatusFound)
}

func (rejector RedirectRejector) Forbidden(w http.ResponseWriter, r *http.Request) {} //noop
