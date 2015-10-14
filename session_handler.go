package auth

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/pivotal-golang/lager"
)

const SessionName = "_concourse_session"
const SessionTokenKey = "token"

type Rejector interface {
	Unauthorized(http.ResponseWriter, *http.Request)
}

type SessionHandler struct {
	Logger lager.Logger

	Handler http.Handler

	Rejector Rejector

	Store sessions.Store
}

func (handler SessionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		auth = handler.readAuthFromSession(r)
	}

	if auth != "" {
		handler.saveAuthToSession(w, r, auth)
		r.Header.Set("Authorization", auth)
	}

	handler.Handler.ServeHTTP(w, r)
}

func (handler SessionHandler) readAuthFromSession(r *http.Request) string {
	session, err := handler.Store.Get(r, SessionName)
	if err != nil {
		// this can happen if they have a session whose secret is no longer valid,
		// so just pretend the session's not there
		return ""
	}

	token, found := session.Values[SessionTokenKey]
	if !found {
		return ""
	}

	t, ok := token.(string)
	if !ok {
		handler.Logger.Info("user-has-token-with-bogus-value")
		return ""
	}

	return t
}

func (handler SessionHandler) saveAuthToSession(w http.ResponseWriter, r *http.Request, auth string) {
	session, err := handler.Store.New(r, SessionName)
	if err != nil {
		// this can happen if they have a session whose secret is no longer valid,
		// so we'll just ignore the error (.New always returns a *Session!)
	}

	session.Values[SessionTokenKey] = auth

	err = session.Save(r, w)
	if err != nil {
		handler.Logger.Error("failed-to-save-session", err)
		return
	}
}
