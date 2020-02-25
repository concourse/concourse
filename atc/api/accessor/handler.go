package accessor

import (
	"context"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/auditor"
)

//go:generate counterfeiter net/http.Handler

//go:generate coonuterfeiter UserTracker

type UserTracker interface {
	CreateOrUpdateUser(username, connector, sub string) error
}

func NewHandler(
	logger lager.Logger,
	handler http.Handler,
	accessFactory AccessFactory,
	action string,
	aud auditor.Auditor,
	userTracker UserTracker,
) http.Handler {
	return accessorHandler{
		logger:        logger,
		handler:       handler,
		accessFactory: accessFactory,
		action:        action,
		auditor:       aud,
		userTracker:   userTracker,
	}
}

type accessorHandler struct {
	logger        lager.Logger
	handler       http.Handler
	accessFactory AccessFactory
	action        string
	auditor       auditor.Auditor
	userTracker   UserTracker
}

func (h accessorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	acc, err := h.accessFactory.Create(r, h.action)
	if err != nil {
		h.logger.Error("failed-to-create-accessor", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	claims := acc.Claims()

	if claims.Sub != "" {
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

func GetAccessor(r *http.Request) Access {
	accessor := r.Context().Value("accessor")
	if accessor != nil {
		return accessor.(Access)
	}

	return &access{}
}
