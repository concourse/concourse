package network

import (
	"fmt"
	"time"

	"github.com/miekg/dns"
)

// DNSServer creates a DNS server that forwards requests to the resolvers
// specified in /etc/resolv.conf.
func DNSServer() (*dns.Server, error) {
	resolvConf, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return nil, err
	}

	return DNSServerWithConfig(resolvConf)
}

// DNSServerWithConfig creates a DNS server that forwards requests to the
// resolvers specified in the provided config.
func DNSServerWithConfig(resolvConf *dns.ClientConfig) (*dns.Server, error) {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		var lastErr error
		for _, server := range resolvConf.Servers {
			// TODO: support the `search` configuration

			// Create a client with timeout based on the config
			client := &dns.Client{
				Timeout: time.Duration(resolvConf.Timeout) * time.Second,
			}

			response, _, err := client.Exchange(r, fmt.Sprintf("%s:%s", server, resolvConf.Port))
			if err == nil {
				response.Compress = true
				_ = w.WriteMsg(response)
				return
			}
			lastErr = err
		}

		// We only get here if all servers failed
		if lastErr != nil {
			m := new(dns.Msg)
			m.SetReply(r)
			m.SetRcode(r, dns.RcodeRefused)
			_ = w.WriteMsg(m)
		}
	})

	return &dns.Server{
		Addr:    ":53",
		Net:     "udp",
		Handler: mux,
	}, nil
}
