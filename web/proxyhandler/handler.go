package proxyhandler

import (
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"code.cloudfoundry.org/lager"
)

func NewHandler(logger lager.Logger, host string) (http.Handler, error) {

	targetUrl, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{
		Timeout:   24 * time.Hour,
		KeepAlive: 24 * time.Hour,
	}

	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		Dial:                dialer.Dial,
		TLSHandshakeTimeout: 60 * time.Second,
	}

	handler := httputil.NewSingleHostReverseProxy(targetUrl)
	handler.FlushInterval = 100 * time.Millisecond
	handler.Transport = transport

	return handler, nil
}
