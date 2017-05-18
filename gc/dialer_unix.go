// +build linux darwin solaris

package gc

import (
	"net"
	"time"

	"github.com/felixge/tcpkeepalive"
)

func keepaliveDialer(addr string) func(string, string) (net.Conn, error) {
	return func(string, string) (net.Conn, error) {
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			return nil, err
		}

		err = tcpkeepalive.SetKeepAlive(conn, 10*time.Second, 3, 5*time.Second)
		if err != nil {
			println("failed to enable connection keepalive: " + err.Error())
		}

		return conn, nil
	}
}
