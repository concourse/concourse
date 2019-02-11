package db

import (
	"net"
	"time"
)

type keepAliveDialer struct {
}

func (d keepAliveDialer) Dial(network, address string) (net.Conn, error) {
	dialer := &net.Dialer{
		KeepAlive: 15 * time.Second,
	}

	return dialer.Dial(network, address)
}

func (d keepAliveDialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	dialer := &net.Dialer{
		KeepAlive: 15 * time.Second,
		Timeout:   timeout,
	}

	return dialer.Dial(network, address)
}
