package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/tedsuo/rata"

	"github.com/concourse/concourse/worker/baggageclaim"
)

type volumeFuture struct {
	client *client
	handle string
}

func (f *volumeFuture) Wait(ctx context.Context) (baggageclaim.Volume, error) {
	request, err := f.client.generateRequest(ctx, baggageclaim.CreateVolumeAsyncCheck, rata.Params{
		"handle": f.handle,
	}, nil)
	if err != nil {
		return nil, err
	}

	exponentialBackoff := backoff.NewExponentialBackOff()
	exponentialBackoff.InitialInterval = 10 * time.Millisecond
	exponentialBackoff.MaxInterval = 10 * time.Second
	exponentialBackoff.MaxElapsedTime = 0

	for {
		response, err := f.client.httpClient(ctx).Do(request)
		if err != nil {
			return nil, err
		}

		if response.StatusCode == http.StatusNoContent {
			response.Body.Close()

			time.Sleep(exponentialBackoff.NextBackOff())

			continue
		}

		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			if response.StatusCode == http.StatusNotFound {
				return nil, fmt.Errorf("future not found: %s", f.handle)
			}
			return nil, getError(response)
		}

		if header := response.Header.Get("Content-Type"); header != "application/json" {
			return nil, fmt.Errorf("unexpected content-type of: %s", header)
		}

		var volumeResponse baggageclaim.VolumeResponse
		err = json.NewDecoder(response.Body).Decode(&volumeResponse)
		if err != nil {
			return nil, err
		}

		return f.client.newVolume(volumeResponse), nil
	}
}

func (f *volumeFuture) Destroy(ctx context.Context) error {
	request, err := f.client.generateRequest(ctx, baggageclaim.CreateVolumeAsyncCancel, rata.Params{
		"handle": f.handle,
	}, nil)
	if err != nil {
		return err
	}

	response, err := f.client.httpClient(ctx).Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusNotFound {
			return fmt.Errorf("future not found: %s", f.handle)
		}
		return getError(response)
	}

	return nil
}
