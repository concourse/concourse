package gc

import (
	"net"
	"time"
)

func keepaliveDialer(addr string) func(string, string) (net.Conn, error) {
	return func(string, string) (net.Conn, error) {
		return net.DialTimeout("tcp", addr, 5*time.Second)
	}
}
