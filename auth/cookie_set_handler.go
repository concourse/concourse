package auth

import (
	"net/http"
	"time"
)

const CookieName = "ATC-Authorization"
const CookieAge = 24 * time.Hour

type CookieSetHandler struct {
	Handler http.Handler
}

func (handler CookieSetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(CookieName)
	if err == nil {
		r.Header.Set("Authorization", cookie.Value)
	}

	handler.Handler.ServeHTTP(w, r)
}
