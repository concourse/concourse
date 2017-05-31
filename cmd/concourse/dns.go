package main

import (
	"fmt"

	"github.com/miekg/dns"
)

type DNSConfig struct {
	Enable bool `long:"enable" description:"Enable proxy DNS server."`
}

func (config DNSConfig) Server() (*dns.Server, error) {
	resolvConf, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return nil, err
	}

	var client dns.Client
	mux := dns.NewServeMux()
	mux.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		for _, server := range resolvConf.Servers {
			response, _, err := client.Exchange(r, fmt.Sprintf("%s:%s", server, resolvConf.Port))
			if err == nil {
				w.WriteMsg(response)
				break
			}
		}

		if err != nil {
			var m *dns.Msg
			m.SetRcode(r, dns.RcodeRefused)
			w.WriteMsg(m)
		}
	})

	return &dns.Server{
		Addr:    ":53",
		Net:     "udp",
		Handler: mux,
	}, nil
}
