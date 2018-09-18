package transport

import "net/http"

type gardenRoundTripper struct {
	db                TransportDB
	workerName        string
	innerRoundTripper http.RoundTripper
	cachedHost        *string
}

func NewGardenRoundTripper(workerName string, workerHost *string, db TransportDB, innerRoundTripper http.RoundTripper) http.RoundTripper {
	return &gardenRoundTripper{
		innerRoundTripper: innerRoundTripper,
		workerName:        workerName,
		db:                db,
		cachedHost:        workerHost,
	}
}

func (c *gardenRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if c.cachedHost == nil {
		savedWorker, found, err := c.db.GetWorker(c.workerName)
		if err != nil {
			return nil, err
		}

		if !found {
			return nil, WorkerMissingError{WorkerName: c.workerName}
		}

		if savedWorker.GardenAddr() == nil {
			return nil, WorkerUnreachableError{
				WorkerName:  c.workerName,
				WorkerState: string(savedWorker.State()),
			}
		}

		c.cachedHost = savedWorker.GardenAddr()
	}

	updatedURL := *request.URL
	updatedURL.Host = *c.cachedHost

	updatedRequest := *request
	updatedRequest.URL = &updatedURL

	response, err := c.innerRoundTripper.RoundTrip(&updatedRequest)
	if err != nil {
		c.cachedHost = nil
	}

	return response, err
}
