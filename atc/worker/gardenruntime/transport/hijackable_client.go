package transport

import (
	"net/http"

	"github.com/concourse/retryhttp"
)

type hijackableClient struct {
	db                    TransportDB
	workerName            string
	innerHijackableClient retryhttp.HijackableClient
	cachedHost            *string
}

func NewHijackableClient(workerName string, db TransportDB, innerHijackableClient retryhttp.HijackableClient) retryhttp.HijackableClient {
	return &hijackableClient{
		innerHijackableClient: innerHijackableClient,
		workerName:            workerName,
		db:                    db,
		cachedHost:            nil,
	}
}

func (c *hijackableClient) Do(request *http.Request) (*http.Response, retryhttp.HijackCloser, error) {
	if c.cachedHost == nil {
		savedWorker, found, err := c.db.GetWorker(c.workerName)
		if err != nil {
			return nil, nil, err
		}

		if !found {
			return nil, nil, WorkerMissingError{WorkerName: c.workerName}
		}

		if savedWorker.GardenAddr() == nil {
			return nil, nil, WorkerUnreachableError{
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

	response, hijackCloser, err := c.innerHijackableClient.Do(&updatedRequest)
	if err != nil {
		c.cachedHost = nil
	}
	return response, hijackCloser, err
}
