package internal

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"log"

	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"
)

//go:generate counterfeiter . Connection

type Connection interface {
	URL() string
	HTTPClient() *http.Client

	Send(request Request, response *Response) error
	ConnectToEventStream(request Request) (*sse.EventSource, error)
}

type Request struct {
	RequestName        string
	Params             rata.Params
	Query              url.Values
	Header             http.Header
	Body               io.Reader
	ReturnResponseBody bool
}

type Response struct {
	Result  interface{}
	Headers *http.Header
	Created bool
}

type connection struct {
	url        string
	httpClient *http.Client
	tracing    bool

	requestGenerator *rata.RequestGenerator
}

func NewConnection(apiURL string, httpClient *http.Client, tracing bool) Connection {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	apiURL = strings.TrimRight(apiURL, "/")
	routes := append(atc.Routes, rata.Route{
		Name:   "ListAuthMethods",
		Path:   "/auth/list_methods",
		Method: "GET",
	}, rata.Route{
		Name:   "GetAuthToken",
		Path:   "/auth/basic/token",
		Method: "GET",
	})

	return &connection{
		url:        apiURL,
		httpClient: httpClient,
		tracing:    tracing,

		requestGenerator: rata.NewRequestGenerator(apiURL, routes),
	}
}

func (connection *connection) URL() string {
	return connection.url
}

func (connection *connection) HTTPClient() *http.Client {
	return connection.httpClient
}

func (connection *connection) Send(passedRequest Request, passedResponse *Response) error {
	req, err := connection.createHTTPRequest(passedRequest)
	if err != nil {
		return err
	}

	if connection.tracing {
		b, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			return err
		}

		log.Println(string(b))
	}

	response, err := connection.httpClient.Do(req)
	if err != nil {
		return err
	}

	if connection.tracing {
		b, err := httputil.DumpResponse(response, true)
		if err != nil {
			return err
		}

		log.Println(string(b))
	}

	if !passedRequest.ReturnResponseBody {
		defer response.Body.Close()
	}

	return connection.populateResponse(response, passedRequest.ReturnResponseBody, passedResponse)
}

func (connection *connection) ConnectToEventStream(passedRequest Request) (*sse.EventSource, error) {
	source, err := sse.Connect(connection.httpClient, time.Second, func() *http.Request {
		request, reqErr := connection.createHTTPRequest(passedRequest)
		if reqErr != nil {
			panic("unexpected error creating request: " + reqErr.Error())
		}

		return request
	})
	if err != nil {
		if brErr, ok := err.(sse.BadResponseError); ok {
			if brErr.Response.StatusCode == http.StatusUnauthorized {
				return nil, ErrUnauthorized
			}
			if brErr.Response.StatusCode == http.StatusForbidden {
				return nil, ErrForbidden
			}
		}

		return nil, err
	}

	return source, nil
}

func (connection *connection) createHTTPRequest(passedRequest Request) (*http.Request, error) {
	body := connection.getBody(passedRequest)

	req, err := connection.requestGenerator.CreateRequest(
		passedRequest.RequestName,
		passedRequest.Params,
		body,
	)
	if err != nil {
		return nil, err
	}

	req.URL.RawQuery = passedRequest.Query.Encode()

	for h, vs := range passedRequest.Header {
		for _, v := range vs {
			req.Header.Add(h, v)
		}
	}

	return req, nil
}

func (connection *connection) getBody(passedRequest Request) io.Reader {
	if passedRequest.Header != nil && passedRequest.Body != nil {
		if _, ok := passedRequest.Header["Content-Type"]; !ok {
			panic("You must pass a 'Content-Type' Header with a body")
		}
		return passedRequest.Body
	}

	return nil
}

func (connection *connection) populateResponse(response *http.Response, returnResponseBody bool, passedResponse *Response) error {
	if response.StatusCode == http.StatusNotFound {
		var errors ResourceNotFoundError

		json.NewDecoder(response.Body).Decode(&errors)
		return errors
	}

	if response.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}

	if response.StatusCode == http.StatusForbidden {
		return ErrForbidden
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(response.Body)

		return UnexpectedResponseError{
			StatusCode: response.StatusCode,
			Status:     response.Status,
			Body:       string(body),
		}
	}

	if passedResponse == nil {
		return nil
	}

	switch response.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusCreated:
		passedResponse.Created = true
	}

	if passedResponse.Headers != nil {
		for k, v := range response.Header {
			(*passedResponse.Headers)[k] = v
		}
	}

	if returnResponseBody {
		passedResponse.Result = response.Body
		return nil
	}

	if passedResponse.Result == nil {
		return nil
	}

	err := json.NewDecoder(response.Body).Decode(passedResponse.Result)
	if err != nil {
		return err
	}

	return nil
}
