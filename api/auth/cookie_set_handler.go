package auth

import (
	"context"
	"net/http"
)

type CookieSetHandler struct {
	Handler http.Handler
}

func (handler CookieSetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(AuthCookieName)
	if err == nil {
		ctx := context.WithValue(r.Context(), CSRFRequiredKey, true)
		r = r.WithContext(ctx)

		if r.Header.Get("Authorization") == "" {
			r.Header.Set("Authorization", cookie.Value)
		}
	}

	handler.Handler.ServeHTTP(w, r)
}
