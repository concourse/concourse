package transport

import (
	"net/http"
	"net/url"

	"github.com/concourse/atc/dbng"
)

type baggageclaimRoundTripper struct {
	db                    TransportDB
	workerName            string
	innerRoundTripper     http.RoundTripper
	cachedBaggageclaimURL *string
}

func NewBaggageclaimRoundTripper(workerName string, baggageclaimURL *string, db TransportDB, innerRoundTripper http.RoundTripper) http.RoundTripper {
	return &baggageclaimRoundTripper{
		innerRoundTripper: innerRoundTripper,
		workerName:        workerName,
		db:                db,
		cachedBaggageclaimURL: baggageclaimURL,
	}
}

func (c *baggageclaimRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if c.cachedBaggageclaimURL == nil {
		savedWorker, found, err := c.db.GetWorker(c.workerName)
		if err != nil {
			return nil, err
		}

		if !found {
			return nil, ErrMissingWorker{WorkerName: c.workerName}
		}

		if savedWorker.State() == dbng.WorkerStateStalled {
			return nil, ErrWorkerStalled{WorkerName: c.workerName}
		}

		if savedWorker.BaggageclaimURL() == nil {
			return nil, ErrWorkerBaggageclaimURLIsMissing{WorkerName: savedWorker.Name()}
		}

		c.cachedBaggageclaimURL = savedWorker.BaggageclaimURL()
	}

	baggageclaimURL, err := url.Parse(*c.cachedBaggageclaimURL)
	if err != nil {
		return nil, err
	}

	updatedURL := *request.URL
	updatedURL.Host = baggageclaimURL.Host

	updatedRequest := *request
	updatedRequest.URL = &updatedURL

	response, err := c.innerRoundTripper.RoundTrip(&updatedRequest)
	if err != nil {
		c.cachedBaggageclaimURL = nil
	}

	return response, err
}
