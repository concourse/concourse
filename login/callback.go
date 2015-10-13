package login

import (
	"fmt"
	"net/http"

	"github.com/concourse/atc/auth"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth/gothic"
	"github.com/pivotal-golang/lager"
)

type callbackHandler struct {
	logger lager.Logger
	store  sessions.Store
}

func NewCallbackHandler(
	logger lager.Logger,
	store sessions.Store,
) http.Handler {
	return &callbackHandler{
		logger: logger,
		store:  store,
	}
}

func (handler *callbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, err := gothic.CompleteUserAuth(w, r)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}

	session, err := handler.store.Get(r, auth.SessionName)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}

	session.Values[auth.SessionTokenKey] = "Token " + user.AccessToken

	err = session.Save(r, w)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}
