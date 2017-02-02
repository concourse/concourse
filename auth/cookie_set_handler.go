package auth

import (
	"net/http"
)

const CookieName = "ATC-Authorization"

type CookieSetHandler struct {
	Handler http.Handler
}

func (handler CookieSetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(CookieName)
	if err == nil && r.Header.Get("Authorization") == "" {
		r.Header.Set("Authorization", cookie.Value)
	}

	handler.Handler.ServeHTTP(w, r)
}
