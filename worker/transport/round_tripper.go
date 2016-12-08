package transport

import "net/http"

//go:generate counterfeiter . RoundTripper

type RoundTripper interface {
	RoundTrip(*http.Request) (*http.Response, error)
}
