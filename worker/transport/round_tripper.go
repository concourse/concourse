package transport

import (
	"net/http"

	"github.com/concourse/atc/dbng"
)

//go:generate counterfeiter . RoundTripper
type RoundTripper interface {
	RoundTrip(*http.Request) (*http.Response, error)
}

type roundTripper struct {
	db                TransportDB
	workerName        string
	innerRoundTripper http.RoundTripper
	cachedHost        string
}

func NewRoundTripper(workerName string, workerHost string, db TransportDB, innerRoundTripper http.RoundTripper) http.RoundTripper {
	return &roundTripper{
		innerRoundTripper: innerRoundTripper,
		workerName:        workerName,
		db:                db,
		cachedHost:        workerHost,
	}
}

func (c *roundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if c.cachedHost == "" {
		savedWorker, found, err := c.db.GetWorker(c.workerName)
		if err != nil {
			return nil, err
		}

		if !found {
			return nil, ErrMissingWorker{WorkerName: c.workerName}
		}

		if savedWorker.State == dbng.WorkerStateStalled {
			return nil, ErrWorkerStalled{WorkerName: c.workerName}
		}

		c.cachedHost = *savedWorker.GardenAddr
	}

	updatedURL := *request.URL
	updatedURL.Host = c.cachedHost

	updatedRequest := *request
	updatedRequest.URL = &updatedURL

	response, err := c.innerRoundTripper.RoundTrip(&updatedRequest)
	if err != nil {
		c.cachedHost = ""
	}

	return response, err
}
