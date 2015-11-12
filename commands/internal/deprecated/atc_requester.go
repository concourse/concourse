package deprecated

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
)

type AtcRequester struct {
	*rata.RequestGenerator
	HttpClient *http.Client
}

func NewAtcRequester(target string, httpClient *http.Client) *AtcRequester {
	return &AtcRequester{
		rata.NewRequestGenerator(target, atc.Routes),
		httpClient,
	}
}
