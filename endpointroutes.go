package main

import (
	"io"
	"net/http"
	"net/url"

	"github.com/tedsuo/router"
)

type EndpointRoutes struct {
	*url.URL
	router.Routes
}

func (endpoint *EndpointRoutes) RequestForHandler(handler string, params router.Params, body io.Reader) (*http.Request, error) {
	req, err := endpoint.Routes.RequestForHandler(handler, params, body)
	if err != nil {
		return nil, err
	}

	req.URL.Scheme = endpoint.URL.Scheme
	req.URL.Host = endpoint.URL.Host

	return req, nil
}
