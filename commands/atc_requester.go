package commands

import (
	"crypto/tls"
	"net/http"

	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
)

type atcRequester struct {
	*rata.RequestGenerator
	httpClient *http.Client
}

func newAtcRequester(target string, insecure bool) *atcRequester {
	tlsClientConfig := &tls.Config{InsecureSkipVerify: insecure}

	return &atcRequester{
		rata.NewRequestGenerator(target, atc.Routes),
		&http.Client{Transport: &http.Transport{TLSClientConfig: tlsClientConfig}},
	}
}
