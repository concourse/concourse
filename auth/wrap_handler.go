package auth

import (
	"net/http"

	"github.com/gorilla/context"
)

var authenticated = "authenticated"
var teamNameKey = "teamName"
var teamIDKey = "teamID"
var isAdminKey = "isAdmin"
var isSystemKey = "system"

func WrapHandler(
	handler http.Handler,
	validator Validator,
	userContextReader UserContextReader,
) http.Handler {
	return authHandler{
		handler:           handler,
		validator:         validator,
		userContextReader: userContextReader,
	}
}

type authHandler struct {
	handler           http.Handler
	validator         Validator
	userContextReader UserContextReader
}

func (h authHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	context.Set(r, authenticated, h.validator.IsAuthenticated(r))
	teamName, teamID, isAdmin, found := h.userContextReader.GetTeam(r)
	if found {
		context.Set(r, teamNameKey, teamName)
		context.Set(r, teamIDKey, teamID)
		context.Set(r, isAdminKey, isAdmin)
	}

	isSystem, found := h.userContextReader.GetSystem(r)
	if found {
		context.Set(r, isSystemKey, isSystem)
	}
	h.handler.ServeHTTP(w, r)
}
