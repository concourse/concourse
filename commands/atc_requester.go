package commands

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
)

type atcRequester struct {
	*rata.RequestGenerator
	httpClient *http.Client
}

func newAtcRequester(target string, httpClient *http.Client) *atcRequester {
	return &atcRequester{
		rata.NewRequestGenerator(target, atc.Routes),
		httpClient,
	}
}
