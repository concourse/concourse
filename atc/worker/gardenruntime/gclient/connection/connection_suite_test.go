package connection_test

import (
	"net"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestConnection(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Connection Suite")
}

func uint64ptr(n uint64) *uint64 {
	return &n
}

type wrappedConnection struct {
	net.Conn

	mu     sync.Mutex
	closed bool
}

func (wc *wrappedConnection) isClosed() bool {
	wc.mu.Lock()
	defer wc.mu.Unlock()

	return wc.closed
}

func (wc *wrappedConnection) Close() error {
	err := wc.Conn.Close()

	wc.mu.Lock()
	defer wc.mu.Unlock()
	if err == nil {
		wc.closed = true
	}
	return err
}
