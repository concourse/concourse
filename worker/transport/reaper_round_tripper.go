package transport

import (
	"net/http"
	"net/url"
)

type reaperRoundTripper struct {
	db                TransportDB
	workerName        string
	innerRoundTripper http.RoundTripper
	cachedreaperURL   *string
}

func NewreaperRoundTripper(workerName string, reaperURL *string, db TransportDB, innerRoundTripper http.RoundTripper) http.RoundTripper {
	return &reaperRoundTripper{
		innerRoundTripper: innerRoundTripper,
		workerName:        workerName,
		db:                db,
		cachedreaperURL:   reaperURL,
	}
}

func (c *reaperRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if c.cachedreaperURL == nil {
		savedWorker, found, err := c.db.GetWorker(c.workerName)
		if err != nil {
			return nil, err
		}

		if !found {
			return nil, WorkerMissingError{WorkerName: c.workerName}
		}

		if savedWorker.ReaperAddr() == nil {
			return nil, WorkerUnreachableError{
				WorkerName:  c.workerName,
				WorkerState: string(savedWorker.State()),
			}
		}

		c.cachedreaperURL = savedWorker.ReaperAddr()
	}

	reaperURL, err := url.Parse(*c.cachedreaperURL)
	if err != nil {
		return nil, err
	}

	updatedURL := *request.URL
	updatedURL.Scheme = reaperURL.Scheme
	updatedURL.Host = reaperURL.Host

	updatedRequest := *request
	updatedRequest.URL = &updatedURL

	response, err := c.innerRoundTripper.RoundTrip(&updatedRequest)
	if err != nil {
		c.cachedreaperURL = nil
	}

	return response, err
}
