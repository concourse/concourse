package network

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDNSServerWithConfig(t *testing.T) {
	// Setup a mock DNS server (upstream)
	mockDNS := startMockDNSServer(t)
	defer mockDNS.server.Shutdown()

	// Extract host and port from mock server address
	mockHost, mockPortStr, err := net.SplitHostPort(mockDNS.server.Addr)
	require.NoError(t, err)

	// Create a DNS client config pointing to our mock server
	mockConfig := &dns.ClientConfig{
		Servers:  []string{mockHost},
		Port:     mockPortStr,
		Ndots:    1,
		Timeout:  5,
		Attempts: 2,
	}

	// Create our DNS server with the mock config
	server, err := DNSServerWithConfig(mockConfig)
	require.NoError(t, err)

	// Change the listen address to avoid port conflicts
	addr, err := getFreeUDPPort()
	require.NoError(t, err)
	server.Addr = addr

	// Start the server
	serverErrCh := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && err.Error() != "server closed" {
			serverErrCh <- err
		}
		close(serverErrCh)
	}()
	defer server.Shutdown()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Check if server started successfully
	select {
	case err := <-serverErrCh:
		require.NoError(t, err, "DNS server failed to start")
	default:
		// Server started successfully
	}

	// Create a client to test the server
	client := new(dns.Client)

	// Test a successful query
	t.Run("Successful query", func(t *testing.T) {
		msg := new(dns.Msg)
		msg.SetQuestion("example.com.", dns.TypeA)
		msg.RecursionDesired = true

		response, _, err := client.Exchange(msg, server.Addr)
		require.NoError(t, err)
		assert.Equal(t, dns.RcodeSuccess, response.Rcode)
		assert.NotEmpty(t, response.Answer)
	})

	// Test error handling
	t.Run("Handle upstream error", func(t *testing.T) {
		// First, wait to ensure no pending requests
		time.Sleep(100 * time.Millisecond)

		// Now set the mock server to fail mode
		mockDNS.failMode = true
		defer func() { mockDNS.failMode = false }()

		// Modify the config to have a very short timeout
		mockConfig.Timeout = 1 // 1 second timeout

		// Wait for fail mode to take effect
		time.Sleep(100 * time.Millisecond)

		msg := new(dns.Msg)
		msg.SetQuestion("example.com.", dns.TypeA)
		msg.RecursionDesired = true

		// Use client with extended timeout
		customClient := &dns.Client{
			Timeout: 2000 * time.Millisecond, // 2 second timeout to receive response
		}

		// Try multiple times if needed (DNS can be flaky in tests)
		var response *dns.Msg
		var err error
		for attempts := 0; attempts < 3; attempts++ {
			response, _, err = customClient.Exchange(msg, server.Addr)
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		require.NoError(t, err, "Failed to get response from server after multiple attempts")
		assert.Equal(t, dns.RcodeRefused, response.Rcode, "Expected REFUSED response")
	})
}

// getFreeUDPPort gets a free UDP port
func getFreeUDPPort() (string, error) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return "", err
	}

	port := fmt.Sprintf("127.0.0.1:%d", conn.LocalAddr().(*net.UDPAddr).Port)
	conn.Close()

	return port, nil
}

// mockDNS holds the mock DNS server and its state
type mockDNS struct {
	server   *dns.Server
	failMode bool
}

// startMockDNSServer starts a mock DNS server that will respond to all queries
func startMockDNSServer(t *testing.T) *mockDNS {
	mock := &mockDNS{
		failMode: false,
	}

	// Find a free port
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	require.NoError(t, err)

	conn, err := net.ListenUDP("udp", addr)
	require.NoError(t, err)

	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()

	serverAddr := fmt.Sprintf("127.0.0.1:%d", port)

	// Configure the mock server
	mux := dns.NewServeMux()
	mux.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		if mock.failMode {
			// Simulate a failure by not responding
			return
		}

		// Create a valid response
		m := new(dns.Msg)
		m.SetReply(r)

		// Set proper fields
		m.Authoritative = true
		m.Rcode = dns.RcodeSuccess

		// Add a fake response record for any A record question
		if len(r.Question) > 0 {
			q := r.Question[0]
			if q.Qtype == dns.TypeA {
				rr, err := dns.NewRR(fmt.Sprintf("%s 3600 IN A 192.0.2.1", q.Name))
				if err == nil {
					m.Answer = append(m.Answer, rr)
				}
			}
		}

		err := w.WriteMsg(m)
		require.NoError(t, err)
	})

	// Create the server
	server := &dns.Server{
		Addr:    serverAddr,
		Net:     "udp",
		Handler: mux,
	}

	// Start listening in a goroutine
	go func() {
		err := server.ListenAndServe()
		if err != nil && err.Error() != "server closed" {
			t.Logf("Mock DNS server error: %v", err)
		}
	}()

	// Wait a moment for the server to start
	time.Sleep(100 * time.Millisecond)

	mock.server = server
	return mock
}
