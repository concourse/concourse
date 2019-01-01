package healthcheck

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
)

type Baggageclaim struct {
	Url string
}

const emptyStrategyPayloadFormat = `{"handle":"%s", "strategy":{"type":"empty"}}`

func (b *Baggageclaim) Create(ctx context.Context, handle string) (*Volume, error) {
	var (
		url    = b.Url + "/volumes"
		method = http.MethodPost
		body   = bytes.NewBufferString(fmt.Sprintf(emptyStrategyPayloadFormat, handle))
		vol    = &Volume{}
	)

	err := doRequest(ctx, method, url, body, vol)
	if err != nil {
		return nil, errors.Wrapf(err,
			"create request failed")
	}

	return vol, nil
}

func (b *Baggageclaim) Destroy(ctx context.Context, handle string) error {
	var (
		url    = b.Url + "/volumes/" + handle
		method = http.MethodDelete
	)

	err := doRequest(ctx, method, url, nil, nil)
	if err != nil {
		return errors.Wrapf(err,
			"destroy request failed")
	}

	return nil
}
