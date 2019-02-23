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

type Garden struct {
	Url string
}

func containerCreationPayload(handle, rootfs string, ttl time.Duration) []byte {
	ttlInSeconds := uint(math.Ceil(ttl.Seconds()))

	payload := fmt.Sprintf(`{"handle":"%s", "rootfs":"raw://%s", "grace_time":%d}`,
		handle, rootfs, ttlInSeconds)

	return []byte(payload)
}

func (g *Garden) Create(ctx context.Context, handle, rootfs string, ttl time.Duration) error {
	var (
		url    = g.Url + "/containers"
		method = http.MethodPost
		body   = bytes.NewBuffer(containerCreationPayload(handle, rootfs, ttl))
	)

	err := doRequest(ctx, method, url, body, nil)
	if err != nil {
		return errors.Wrapf(err,
			"create request failed")
	}

	return nil
}
