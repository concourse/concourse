package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"code.cloudfoundry.org/lager/v3"
)

type componentHealth struct {
	Healthy       bool   `json:"healthy"`
	ResponseError string `json:"response_error"`
}

type healthResponse struct {
	Garden       componentHealth `json:"garden"`
	Baggageclaim componentHealth `json:"baggageclaim"`
}

type healthChecker struct {
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
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(h.timeout))
	defer cancel()

	resp := healthResponse{
		Garden:       componentHealth{Healthy: true},
		Baggageclaim: componentHealth{Healthy: true},
	}

	var gardenErr, baggageclaimErr error
	var wg sync.WaitGroup

	wg.Go(func() {
		gardenErr = doRequest(ctx, h.gardenAddr+"/ping")
	})

	wg.Go(func() {
		baggageclaimErr = doRequest(ctx, h.baggageclaimAddr+"/volumes")
	})

	wg.Wait()

	if gardenErr != nil {
		resp.Garden.Healthy = false
		resp.Garden.ResponseError = gardenErr.Error()
		h.logger.Error("failed-to-ping-garden-server", gardenErr)
	}

	if baggageclaimErr != nil {
		resp.Baggageclaim.Healthy = false
		resp.Baggageclaim.ResponseError = baggageclaimErr.Error()
		h.logger.Error("failed-to-list-volumes-on-baggageclaim-server", baggageclaimErr)
	}

	w.Header().Set("Content-Type", "application/json")
	if !resp.Garden.Healthy || !resp.Baggageclaim.Healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(resp)
}
