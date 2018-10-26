// +build linux darwin solaris

package tsa

import (
	"net"
	"time"

	"github.com/felixge/tcpkeepalive"
)

func keepaliveDialer(network string, address string, dialTimeout time.Duration, idleTimeout time.Duration) (net.Conn, error) {

	conn, err := net.DialTimeout(network, address, dialTimeout)
	if err != nil {
		return nil, err
	}

	err = tcpkeepalive.SetKeepAlive(conn, 10*time.Second, 3, 5*time.Second)
	if err != nil {
		println("failed to enable connection keepalive: " + err.Error())
	}

	if idleTimeout != 0 {
		conn = &timeoutConn{
			Conn:        conn,
			IdleTimeout: idleTimeout,
		}
	}

	return conn, nil
}
