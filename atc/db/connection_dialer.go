package db

import (
	"github.com/felixge/tcpkeepalive"
	"net"
	"time"
)

type keepAliveDialer struct {
	keepAliveIdleTime time.Duration
	keepAliveCount    int
	keepAliveInterval time.Duration
}

func (d keepAliveDialer) Dial(network, address string) (net.Conn, error) {
	c, err := net.Dial(network, address)
	if err != nil || c == nil {
		return c, err
	}

	// Only enable this feature when explicitly set both:
	// * keepAliveIdleTime
	// * keepAliveCount
	// * keepAliveInterval
	if d.keepAliveIdleTime != 0 && d.keepAliveCount != 0 && d.keepAliveInterval != 0 {
		err = tcpkeepalive.SetKeepAlive(c, d.keepAliveIdleTime, d.keepAliveCount, d.keepAliveInterval)
		if err != nil {
			return c, err
		}
	}

	return c, nil
}

func (d keepAliveDialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	c, err := net.DialTimeout(network, address, timeout)
	if err != nil || c == nil {
		return c, err
	}

	// Only enable this feature when explicitly set both:
	// * keepAliveIdleTime
	// * keepAliveCount
	// * keepAliveInterval
	if d.keepAliveIdleTime != 0 && d.keepAliveCount != 0 && d.keepAliveInterval != 0 {
		err = tcpkeepalive.SetKeepAlive(c, d.keepAliveIdleTime, d.keepAliveCount, d.keepAliveInterval)
		if err != nil {
			return c, err
		}
	}

	return c, nil
}
