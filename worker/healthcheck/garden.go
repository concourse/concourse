package healthcheck

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
)

const containerPayloadFormat = `{"handle":"%s", "rootfs":"raw://%s"}`

type Garden struct {
	Url string
}

func (g *Garden) Create(ctx context.Context, handle, rootfs string) error {
	var (
		url    = g.Url + "/containers"
		method = http.MethodPost
		body   = bytes.NewBufferString(fmt.Sprintf(containerPayloadFormat, handle, rootfs))
	)

	err := doRequest(ctx, method, url, body, nil)
	if err != nil {
		return errors.Wrapf(err,
			"create request failed")
	}

	return nil
}

func (g *Garden) Destroy(ctx context.Context, handle string) error {
	var (
		url    = g.Url + "/containers/" + handle
		method = http.MethodDelete
	)

	err := doRequest(ctx, method, url, nil, nil)
	if err != nil {
		return errors.Wrapf(err,
			"destroy request failed")
	}

	return nil
}
