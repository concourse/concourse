package beacon

import (
	"net"
	"time"
)

func keepaliveDialer(network string, address string, timeout time.Duration, idleTimeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout(network, address, timeout)
	if err != nil {
		return nil, err
	}

	if idleTimeout != 0 {
		conn = &ConnectionWithIdleTimeout{
			Conn:        conn,
			IdleTimeout: idleTimeout,
		}
	}

	return conn, nil
}
