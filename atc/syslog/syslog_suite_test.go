package syslog_test

import (
	"crypto/tls"
	"io"
	"net"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSyslog(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Syslog Suite")
}

type testServer struct {
	Addr     string
	Messages chan string

	ln     net.Listener
	closed bool
	wg     *sync.WaitGroup
	mu     sync.RWMutex
}

func newTestServer(cert *tls.Certificate) *testServer {
	server := &testServer{
		Messages: make(chan string, 20),

		wg: new(sync.WaitGroup),
	}

	server.ListenTCP(cert)

	server.wg.Add(1)
	go server.ServeTCP()

	return server
}

func (server *testServer) ListenTCP(cert *tls.Certificate) net.Listener {
	var ln net.Listener

	var err error
	if cert != nil {
		config := &tls.Config{
			Certificates: []tls.Certificate{*cert},
		}
		server.ln, err = tls.Listen("tcp", "127.0.0.1:0", config)
		Expect(err).NotTo(HaveOccurred())
	} else {
		server.ln, err = net.Listen("tcp", "[::]:0")
		Expect(err).NotTo(HaveOccurred())
	}

	Expect(err).NotTo(HaveOccurred())

	server.Addr = server.ln.Addr().String()

	return ln
}

func (server *testServer) ServeTCP() {
	defer server.wg.Done()
	defer GinkgoRecover()

	for {
		conn, err := server.ln.Accept()

		server.mu.RLock()
		if server.closed {
			server.mu.RUnlock()
			return
		}
		server.mu.RUnlock()

		Expect(err).NotTo(HaveOccurred())

		time.Sleep(100 * time.Millisecond)

		buf := make([]byte, 1024)
		n, err := conn.Read(buf)

		// expect bad certificate from 'bad cert' test
		if err != nil && err.Error() == "remote error: tls: bad certificate" {
			continue
		}

		// expect no message from 'open and close' tests
		if err != nil && err == io.EOF {
			continue
		}

		Expect(err).NotTo(HaveOccurred())

		server.Messages <- string(buf[0:n])
	}
}

func (server *testServer) Close() {
	server.mu.Lock()
	server.closed = true
	server.mu.Unlock()

	server.ln.Close()
	server.wg.Wait()
}
