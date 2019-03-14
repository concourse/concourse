package tsa

import (
	"net"
	"time"
)

type timeoutConn struct {
	net.Conn
	IdleTimeout time.Duration
}

func (c *timeoutConn) Write(p []byte) (int, error) {
	c.updateDeadline()
	return c.Conn.Write(p)
}

func (c *timeoutConn) Read(b []byte) (int, error) {
	c.updateDeadline()
	return c.Conn.Read(b)
}

func (c *timeoutConn) updateDeadline() {
	idleDeadline := time.Now().Add(c.IdleTimeout)
	c.Conn.SetDeadline(idleDeadline)
}
