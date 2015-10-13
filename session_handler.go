package auth

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/pivotal-golang/lager"
)

const SessionName = "_concourse_session"
const SessionTokenKey = "token"

type SessionHandler struct {
	Logger lager.Logger

	Handler http.Handler

	Store sessions.Store
}

func (handler SessionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session, err := handler.Store.Get(r, SessionName)
	if err != nil {
		handler.Logger.Info("failed-to-get-session", lager.Data{
			"error": err.Error(),
		})
		Unauthorized(w)
		return
	}

	auth := r.Header.Get("Authorization")
	if auth == "" {
		token, found := session.Values[SessionTokenKey]
		if found {
			token, ok := token.(string)
			if !ok {
				handler.Logger.Info("user-has-sketchy-token")
				Unauthorized(w)
				return
			}

			auth = token
		}
	}

	if auth != "" {
		session.Values[SessionTokenKey] = auth

		err = session.Save(r, w)
		if err != nil {
			handler.Logger.Error("failed-to-save-session", err)
			Unauthorized(w)
			return
		}

		r.Header.Set("Authorization", auth)
	}

	handler.Handler.ServeHTTP(w, r)
}
