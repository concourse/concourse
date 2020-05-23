package accessor

import (
	"context"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/auditor"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter net/http.Handler

//go:generate counterfeiter . AccessFactory

type AccessFactory interface {
	Create(string, Verification, []db.Team) Access
}

//go:generate counterfeiter . TokenVerifier

type TokenVerifier interface {
	Verify(*http.Request) (map[string]interface{}, error)
}

//go:generate counterfeiter .  TeamFetcher

type TeamFetcher interface {
	GetTeams() ([]db.Team, error)
}

//go:generate counterfeiter . UserTracker

type UserTracker interface {
	CreateOrUpdateUser(username, connector, sub string) error
}

func NewHandler(
	logger lager.Logger,
	action string,
	handler http.Handler,
	accessFactory AccessFactory,
	tokenVerifier TokenVerifier,
	teamFetcher TeamFetcher,
	userTracker UserTracker,
	auditor auditor.Auditor,
	customRoles map[string]string,
) http.Handler {
	return &accessorHandler{
		logger:        logger,
		handler:       handler,
		accessFactory: accessFactory,
		action:        action,
		auditor:       auditor,
		tokenVerifier: tokenVerifier,
		teamFetcher:   teamFetcher,
		userTracker:   userTracker,
		customRoles:   customRoles,
	}
}

type accessorHandler struct {
	logger        lager.Logger
	action        string
	handler       http.Handler
	accessFactory AccessFactory
	tokenVerifier TokenVerifier
	teamFetcher   TeamFetcher
	userTracker   UserTracker
	auditor       auditor.Auditor
	customRoles   map[string]string
}

func (h *accessorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	teams, err := h.teamFetcher.GetTeams()
	if err != nil {
		h.logger.Error("failed-to-fetch-teams", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	requiredRole := h.customRoles[h.action]
	if requiredRole == "" {
		requiredRole = DefaultRoles[h.action]
	}

	acc := h.accessFactory.Create(requiredRole, h.verifyToken(r), teams)

	claims := acc.Claims()

	if acc.IsAuthenticated() {

		err = h.userTracker.CreateOrUpdateUser(
			claims.UserName,
			claims.Connector,
			claims.Sub,
		)
		if err != nil {
			h.logger.Error("failed-to-track-user", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	ctx := context.WithValue(r.Context(), "accessor", acc)

	h.auditor.Audit(h.action, claims.UserName, r)
	h.handler.ServeHTTP(w, r.WithContext(ctx))
}

func (h *accessorHandler) verifyToken(r *http.Request) Verification {
	claims, err := h.tokenVerifier.Verify(r)
	if err != nil {
		switch err {
		case ErrVerificationNoToken:
			return Verification{HasToken: false, IsTokenValid: false}
		default:
			return Verification{HasToken: true, IsTokenValid: false}
		}
	}

	return Verification{HasToken: true, IsTokenValid: true, RawClaims: claims}
}

func GetAccessor(r *http.Request) Access {
	accessor := r.Context().Value("accessor")
	if accessor != nil {
		return accessor.(Access)
	}

	return &access{}
}
