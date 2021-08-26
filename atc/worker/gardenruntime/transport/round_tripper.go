package transport

import "net/http"

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . RoundTripper
type RoundTripper interface {
	RoundTrip(*http.Request) (*http.Response, error)
}
