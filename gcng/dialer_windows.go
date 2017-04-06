package gcng

import (
	"net"
	"time"
)

func keepaliveDialer(network string, address string) (net.Conn, error) {
	return net.DialTimeout(network, address, 5*time.Second)
}
