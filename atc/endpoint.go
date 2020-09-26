package atc

import (
	"io"
	"net/http"

	"github.com/tedsuo/rata"
)

type Endpoint interface {
	CreateRequest(string, map[string]string, io.Reader) (*http.Request, error)
}

func NewEndpoint(host string) Endpoint {
	return &rataEndpoint{rata.NewRequestGenerator(host, Routes)}
}

type rataEndpoint struct {
	*rata.RequestGenerator
}

func (rae *rataEndpoint) CreateRequest(
	action string,
	params map[string]string,
	body io.Reader,
) (*http.Request, error) {
	return rae.RequestGenerator.CreateRequest(
		action,
		rata.Params(params),
		body,
	)
}

func CreatePathForRoute(action string, params map[string]string) (string, error) {
	return Routes.CreatePathForRoute(action, rata.Params(params))
}
