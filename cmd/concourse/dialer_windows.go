package main

import (
	"net"
	"time"
)

func keepaliveDialer(network string, address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout(network, address, timeout)
}
