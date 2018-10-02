package beacon

import (
	"net"
	"time"
)

type ConnectionWithIdleTimeout struct {
	net.Conn
	IdleTimeout time.Duration
}

func (c *ConnectionWithIdleTimeout) Write(p []byte) (int, error) {
	c.updateDeadline()
	return c.Conn.Write(p)
}

func (c *ConnectionWithIdleTimeout) Read(b []byte) (int, error) {
	c.updateDeadline()
	return c.Conn.Read(b)
}

func (c *ConnectionWithIdleTimeout) updateDeadline() {
	idleDeadline := time.Now().Add(c.IdleTimeout)
	c.Conn.SetDeadline(idleDeadline)
}