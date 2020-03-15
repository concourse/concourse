package auth

import (
	"context"
	"net/http"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . DefaultBadgeVisibilityFactory
type DefaultBadgeVisibilityFactory interface {
	NewDefaultBadgeVisibility() DefaultBadgeVisibility
}

type defaultBadgeVisibilityFactory struct {
	visibility string
}

func NewDefaultBadgeVisibilityFactory(
	visibility string,
) *defaultBadgeVisibilityFactory {
	switch visibility {
	case "public":
	case "pipeline-visibility":
	default:
		panic("invalid badge visibility type")
	}
	return &defaultBadgeVisibilityFactory{
		visibility: visibility,
	}
}

func (f *defaultBadgeVisibilityFactory) NewDefaultBadgeVisibility() DefaultBadgeVisibility {
	return NewDefaultBadgeVisibility(f.visibility)
}

func NewDefaultBadgeVisibility(
	visibility string,
) DefaultBadgeVisibility {
	return DefaultBadgeVisibility{
		visibility: visibility,
	}
}

type DefaultBadgeVisibility struct {
	visibility string
}

func (dbv DefaultBadgeVisibility) IsVisible() bool {
	switch dbv.visibility {
	case "public":
		return true
	case "pipeline-visibility":
		return false
	default:
		return false
	}
}

type CheckBadgeAccessHandlerFactory interface {
	HandlerFor(pipelineScopedHandler http.Handler, rejector Rejector) http.Handler
}

type checkBadgeAccessHandlerFactory struct {
	teamFactory                   db.TeamFactory
	defaultBadgeVisibilityFactory DefaultBadgeVisibilityFactory
}

func NewCheckBadgeAccessHandlerFactory(
	teamFactory db.TeamFactory,
	defaultBadgeVisibilityFactory DefaultBadgeVisibilityFactory,
) *checkBadgeAccessHandlerFactory {
	return &checkBadgeAccessHandlerFactory{
		teamFactory:                   teamFactory,
		defaultBadgeVisibilityFactory: defaultBadgeVisibilityFactory,
	}
}

func (f *checkBadgeAccessHandlerFactory) HandlerFor(
	delegateHandler http.Handler,
	rejector Rejector,
) http.Handler {
	return checkBadgeAccessHandler{
		rejector:                      rejector,
		teamFactory:                   f.teamFactory,
		defaultBadgeVisibilityFactory: f.defaultBadgeVisibilityFactory,
		delegateHandler:               delegateHandler,
	}
}

type checkBadgeAccessHandler struct {
	rejector                      Rejector
	teamFactory                   db.TeamFactory
	defaultBadgeVisibilityFactory DefaultBadgeVisibilityFactory
	delegateHandler               http.Handler
}

func (h checkBadgeAccessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	teamName := r.FormValue(":team_name")
	pipelineName := r.FormValue(":pipeline_name")

	team, found, err := h.teamFactory.FindTeam(teamName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	pipeline, found, err := team.Pipeline(pipelineName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	defaultBadgeVisibility := h.defaultBadgeVisibilityFactory.NewDefaultBadgeVisibility()

	acc := accessor.GetAccessor(r)

	if acc.IsAuthorized(teamName) || pipeline.Public() || defaultBadgeVisibility.IsVisible() {
		ctx := context.WithValue(r.Context(), PipelineContextKey, pipeline)
		h.delegateHandler.ServeHTTP(w, r.WithContext(ctx))
		return
	}

	if !acc.IsAuthenticated() {
		h.rejector.Unauthorized(w, r)
		return
	}

	h.rejector.Forbidden(w, r)
}
