package accessor

import (
	"context"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/auditor"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter net/http.Handler

func NewHandler(
	logger lager.Logger,
	handler http.Handler,
	accessFactory AccessFactory,
	action string,
	aud auditor.Auditor,
	userFactory db.UserFactory,
) http.Handler {
	return accessorHandler{
		logger:        logger,
		handler:       handler,
		accessFactory: accessFactory,
		action:        action,
		auditor:       aud,
		userFactory:   userFactory,
	}
}

type accessorHandler struct {
	logger        lager.Logger
	handler       http.Handler
	accessFactory AccessFactory
	action        string
	auditor       auditor.Auditor
	userFactory   db.UserFactory
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
		_, err = h.userFactory.CreateOrUpdateUser(
			claims.UserName,
			claims.Connector,
			claims.Sub,
		)
		if err != nil {
			h.logger.Error("failed-to-update-user-activity", err)
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
