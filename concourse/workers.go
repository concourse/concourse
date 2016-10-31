package concourse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
	"github.com/tedsuo/rata"
)

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
