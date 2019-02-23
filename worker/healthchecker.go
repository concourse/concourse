package worker

import (
	"context"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/worker/healthcheck"
)

type healthChecker struct {
	checker healthcheck.Checker
	timeout time.Duration
	logger  lager.Logger
}

func NewHealthChecker(logger lager.Logger, baggageclaimUrl, gardenUrl string, checkTimeout time.Duration) healthChecker {
	checker := &healthcheck.Worker{
		VolumeProvider:    &healthcheck.Baggageclaim{Url: baggageclaimUrl},
		ContainerProvider: &healthcheck.Garden{Url: gardenUrl},
		TTL:               checkTimeout,
	}

	return healthChecker{
		logger:  logger,
		checker: checker,
		timeout: checkTimeout,
	}
}

func (h *healthChecker) CheckHealth(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(h.timeout))
	defer cancel()

	err := h.checker.Check(ctx)
	if err != nil {
		w.WriteHeader(503)
		h.logger.Error("worker-healthcheck-failed", err)
		return
	}
}
