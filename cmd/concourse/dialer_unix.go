// +build linux darwin solaris

package main

import (
	"net"
	"time"

	"github.com/felixge/tcpkeepalive"
)

func keepaliveDialer(network string, address string, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout(network, address, timeout)
	if err != nil {
		return nil, err
	}

	err = tcpkeepalive.SetKeepAlive(conn, 10*time.Second, 3, 5*time.Second)
	if err != nil {
		println("failed to enable connection keepalive: " + err.Error())
	}

	return conn, nil
}
