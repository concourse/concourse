package db

import (
	"net"
	"time"
)

type nilConnErr struct{}

func (e nilConnErr) Error() string {
	return "Connection is nil"
}

type timeoutConn struct {
	conn         net.Conn
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func (t *timeoutConn) Read(b []byte) (n int, err error) {
	if t.conn != nil {
		if t.readTimeout != 0 {
			t.conn.SetReadDeadline(time.Now().Add(t.readTimeout))
		}
		n, err = t.conn.Read(b)
		if t.readTimeout != 0 {
			t.conn.SetReadDeadline(time.Time{})
		}
		return
	}
	return 0, nilConnErr{}
}

func (t *timeoutConn) Write(b []byte) (n int, err error) {
	if t.conn != nil {
		if t.writeTimeout != 0 {
			t.conn.SetWriteDeadline(time.Now().Add(t.writeTimeout))
		}
		n, err = t.conn.Write(b)
		if t.writeTimeout != 0 {
			t.conn.SetWriteDeadline(time.Time{})
		}
		return
	}
	return 0, nilConnErr{}
}

func (t *timeoutConn) Close() (err error) {
	if t.conn != nil {
		err = t.conn.Close()
		if err == nil {
			t.conn = nil
		}
		return
	}
	return nilConnErr{}
}

func (t *timeoutConn) LocalAddr() net.Addr {
	if t.conn != nil {
		return t.conn.LocalAddr()
	}
	return nil
}

func (t *timeoutConn) RemoteAddr() net.Addr {
	if t.conn != nil {
		return t.conn.RemoteAddr()
	}
	return nil
}

func (t *timeoutConn) SetDeadline(time time.Time) error {
	if t.conn != nil {
		return t.conn.SetDeadline(time)
	}
	return nilConnErr{}
}

func (t *timeoutConn) SetReadDeadline(time time.Time) error {
	if t.conn != nil {
		return t.conn.SetReadDeadline(time)
	}
	return nilConnErr{}
}

func (t *timeoutConn) SetWriteDeadline(time time.Time) error {
	if t.conn != nil {
		return t.conn.SetWriteDeadline(time)
	}
	return nilConnErr{}
}

type timeoutDialer struct {
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func (d timeoutDialer) Dial(ntw, addr string) (net.Conn, error) {
	c, err := net.Dial(ntw, addr)
	if err != nil || c == nil {
		return c, err
	}
	return &timeoutConn{conn: c, readTimeout: d.readTimeout, writeTimeout: d.writeTimeout}, nil
}

func (d timeoutDialer) DialTimeout(ntw, addr string, timeout time.Duration) (net.Conn, error) {
	c, err := net.DialTimeout(ntw, addr, timeout)
	if err != nil || c == nil {
		return c, err
	}

	return &timeoutConn{conn: c, readTimeout: d.readTimeout, writeTimeout: d.writeTimeout}, nil
}
