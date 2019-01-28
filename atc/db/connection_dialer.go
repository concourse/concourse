package db

import (
	"net"
	"time"
)

type timeoutConn struct {
	net.Conn
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func (t *timeoutConn) Read(b []byte) (n int, err error) {
	if t.readTimeout != 0 {
		_ = t.Conn.SetReadDeadline(time.Now().Add(t.readTimeout))
	}
	n, err = t.Conn.Read(b)
	if t.readTimeout != 0 {
		_ = t.Conn.SetReadDeadline(time.Time{})
	}
	return n, err
}

func (t *timeoutConn) Write(b []byte) (n int, err error) {
	if t.writeTimeout != 0 {
		_ = t.Conn.SetWriteDeadline(time.Now().Add(t.writeTimeout))
	}
	n, err = t.Conn.Write(b)
	if t.writeTimeout != 0 {
		_ = t.Conn.SetWriteDeadline(time.Time{})
	}
	return n, err
}

type timeoutDialer struct {
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func (d timeoutDialer) Dial(network, address string) (net.Conn, error) {
	c, err := net.Dial(network, address)
	if err != nil || c == nil {
		return c, err
	}
	return &timeoutConn{Conn: c, readTimeout: d.readTimeout, writeTimeout: d.writeTimeout}, nil
}

func (d timeoutDialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	c, err := net.DialTimeout(network, address, timeout)
	if err != nil || c == nil {
		return c, err
	}

	return &timeoutConn{Conn: c, readTimeout: d.readTimeout, writeTimeout: d.writeTimeout}, nil
}
