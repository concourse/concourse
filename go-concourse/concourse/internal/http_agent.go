package internal

import (
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/tedsuo/rata"
)

type HTTPAgent interface {
	Send(request Request) (http.Response, error)
}

type httpAgent struct {
	url        string
	httpClient *http.Client
	tracing    bool

	requestGenerator *rata.RequestGenerator
}

func NewHTTPAgent(apiURL string, httpClient *http.Client, tracing bool) HTTPAgent {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	apiURL = strings.TrimRight(apiURL, "/")

	return &httpAgent{
		url:        apiURL,
		httpClient: httpClient,
		tracing:    tracing,

		requestGenerator: rata.NewRequestGenerator(apiURL, atc.Routes),
	}
}

func (a *httpAgent) Send(request Request) (http.Response, error) {

	req, err := a.createHTTPRequest(request)
	if err != nil {
		return http.Response{}, err
	}

	return a.send(req)
}

func (a *httpAgent) send(req *http.Request) (http.Response, error) {
	if a.tracing {
		b, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			return http.Response{}, err
		}

		log.Println(string(b))
	}

	response, err := a.httpClient.Do(req)
	if err != nil {
		return http.Response{}, err
	}

	if a.tracing {
		b, err := httputil.DumpResponse(response, true)
		if err != nil {
			return http.Response{}, err
		}

		log.Println(string(b))
	}

	showPolicyCheckWarningIfHas(response)

	return *response, nil
}

func (a *httpAgent) createHTTPRequest(request Request) (*http.Request, error) {
	body := a.getBody(request)

	req, err := a.requestGenerator.CreateRequest(
		request.RequestName,
		request.Params,
		body,
	)
	if err != nil {
		return nil, err
	}

	req.URL.RawQuery = request.Query.Encode()

	for h, vs := range request.Header {
		for _, v := range vs {
			req.Header.Add(h, v)
		}
	}

	return req, nil
}

func (a *httpAgent) getBody(request Request) io.Reader {
	if request.Header != nil && request.Body != nil {
		if _, ok := request.Header["Content-Type"]; !ok {
			panic("You must pass a 'Content-Type' Header with a body")
		}
		return request.Body
	}

	return nil
}
