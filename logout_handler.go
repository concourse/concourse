package auth

import (
	"net/http"

	"code.cloudfoundry.org/lager"
)

type LogOutHandler struct {
	logger lager.Logger
}

func NewLogOutHandler(logger lager.Logger) http.Handler {
	return &LogOutHandler{logger: logger}
}

func (handler *LogOutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler.logger.Session("logout")

	http.SetCookie(w, &http.Cookie{
		Name:   AuthCookieName,
		Path:   "/",
		MaxAge: -1,
	})
}
