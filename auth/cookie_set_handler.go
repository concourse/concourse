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
	auth := r.Header.Get("Authorization")
	if auth == "" {
		cookie, err := r.Cookie(CookieName)
		if err == nil {
			auth = cookie.Value
		}
	}

	if auth != "" {
		http.SetCookie(w, &http.Cookie{
			Name:    CookieName,
			Value:   auth,
			Path:    "/",
			Expires: time.Now().Add(CookieAge),
		})

		r.Header.Set("Authorization", auth)
	}

	handler.Handler.ServeHTTP(w, r)
}
