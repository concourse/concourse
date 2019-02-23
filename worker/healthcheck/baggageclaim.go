package healthcheck

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

type Baggageclaim struct {
	Url string
}

func volumeCreationPayload(handle string, ttl time.Duration) []byte {
	ttlInSeconds := uint(math.Ceil(ttl.Seconds()))

	payload := fmt.Sprintf(`{"handle":"%s", "ttl":%d, "strategy":{"type":"empty"}}`,
		handle, ttlInSeconds)

	return []byte(payload)

}

func (b *Baggageclaim) Create(ctx context.Context, handle string, ttl time.Duration) (*Volume, error) {
	var (
		url    = b.Url + "/volumes"
		method = http.MethodPost
		body   = bytes.NewBuffer(volumeCreationPayload(handle, ttl))
		vol    = &Volume{}
	)

	err := doRequest(ctx, method, url, body, vol)
	if err != nil {
		return nil, errors.Wrapf(err,
			"create request failed")
	}

	return vol, nil
}
