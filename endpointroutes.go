package main

import (
	"io"
	"net/http"

	"github.com/tedsuo/router"
)

type EndpointRoutes struct {
	Scheme string
	Host   string

	router.Routes
}

func (endpoint *EndpointRoutes) RequestForHandler(handler string, params router.Params, body io.Reader) (*http.Request, error) {
	req, err := endpoint.Routes.RequestForHandler(handler, params, body)
	if err != nil {
		return nil, err
	}

	req.URL.Scheme = endpoint.Scheme
	req.URL.Host = endpoint.Host

	return req, nil
}
