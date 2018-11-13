package worker

import (
	"context"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
)

type healthChecker struct {
	client           *http.Client
	baggageclaimAddr string
	gardenAddr       string
	timeout          time.Duration
	logger           lager.Logger
}

func NewHealthChecker(logger lager.Logger, baggageclaimAddr, gardenAddr string, checkTimeout time.Duration) healthChecker {
	return healthChecker{
		logger:           logger,
		baggageclaimAddr: baggageclaimAddr,
		gardenAddr:       gardenAddr,
		timeout:          checkTimeout,
	}
}

func doRequest(ctx context.Context, url string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	_ = resp.Body.Close()
	return nil
}

func (h *healthChecker) CheckHealth(w http.ResponseWriter, req *http.Request) {
	var err error

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(h.timeout))
	defer cancel()

	err = doRequest(ctx, h.gardenAddr+"/ping")
	if err != nil {
		w.WriteHeader(503)
		h.logger.Error("failed-to-ping-garden-server", err)
		return
	}

	err = doRequest(ctx, h.baggageclaimAddr+"/volumes")
	if err != nil {
		w.WriteHeader(503)
		h.logger.Error("failed-to-list-volumes-on-baggageclaim-server", err)
		return
	}

	return
}
