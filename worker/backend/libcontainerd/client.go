package libcontainerd

import (
	"context"
	"fmt"

	"github.com/containerd/containerd"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Client

type Client interface {
	Version(ctx context.Context) (err error)
}

type client struct {
	containerd *containerd.Client
}

func New(address string) (c *client, err error) {
	c = new(client)

	c.containerd, err = containerd.New(address)
	if err != nil {
		err = fmt.Errorf("failed to connect to socket: %w", err)
		return
	}

	return
}

// this could be used as `ping`?
//
func (c *client) Version(ctx context.Context) (err error) {
	_, err = c.containerd.Version(ctx)
	return
}
