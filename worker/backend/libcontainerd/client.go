package libcontainerd

import (
	"context"
	"fmt"

	"github.com/containerd/containerd"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Client

type Client interface {
	Init() (err error)
	Version(ctx context.Context) (err error)
}

type client struct {
	addr       string
	containerd *containerd.Client
}

func New(addr string) *client {
	return &client{
		addr: addr,
	}
}

func (c *client) Init() (err error) {
	c.containerd, err = containerd.New(c.addr)
	if err != nil {
		err = fmt.Errorf("failed to connect to addr %s: %w", c.addr, err)
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
