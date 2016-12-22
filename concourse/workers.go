package concourse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

type PruneWorkerError struct {
	atc.PruneWorkerResponseBody
}

func (e PruneWorkerError) Error() string {
	return e.Stderr
}

func (client *client) ListWorkers() ([]atc.Worker, error) {
	var workers []atc.Worker
	err := client.connection.Send(internal.Request{
		RequestName: atc.ListWorkers,
	}, &internal.Response{
		Result: &workers,
	})
	return workers, err
}

func (client *client) SaveWorker(worker atc.Worker, ttl *time.Duration) (*atc.Worker, error) {
	buffer := &bytes.Buffer{}
	err := json.NewEncoder(buffer).Encode(worker)
	if err != nil {
		return nil, fmt.Errorf("Unable to marshal worker: %s", err)
	}

	params := rata.Params{}
	if ttl != nil {
		params["ttl"] = ttl.String()
	}

	var savedWorker *atc.Worker
	err = client.connection.Send(internal.Request{
		RequestName: atc.RegisterWorker,
		Body:        buffer,
		Params:      params,
	}, &internal.Response{
		Result: &savedWorker,
	})

	return savedWorker, err
}

func (client *client) PruneWorker(workerName string) error {
	params := rata.Params{"worker_name": workerName}
	err := client.connection.Send(internal.Request{
		RequestName: atc.PruneWorker,
		Params:      params,
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	}, nil)

	if unexpectedResponseError, ok := err.(internal.UnexpectedResponseError); ok {
		if unexpectedResponseError.StatusCode == http.StatusBadRequest {
			var pruneWorkerErr PruneWorkerError

			err = json.Unmarshal([]byte(unexpectedResponseError.Body), &pruneWorkerErr)
			if err != nil {
				return err
			}

			return pruneWorkerErr
		}
	}

	return err
}
