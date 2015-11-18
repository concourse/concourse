package concourse

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

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
	Body               *bytes.Buffer
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

	requestGenerator *rata.RequestGenerator
}

func NewConnection(apiURL string, httpClient *http.Client) (Connection, error) {
	if apiURL == "" {
		return nil, errors.New("API is blank")
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	apiURL = strings.TrimRight(apiURL, "/")

	return &connection{
		url:        apiURL,
		httpClient: httpClient,

		requestGenerator: rata.NewRequestGenerator(apiURL, atc.Routes),
	}, nil
}

func (connection *connection) URL() string {
	return connection.url
}

func (connection *connection) HTTPClient() *http.Client {
	return connection.httpClient
}

func (connection *connection) Send(passedRequest Request, passedResponse *Response) error {
	req, err := connection.createHTTPRequest(passedRequest)

	response, err := connection.httpClient.Do(req)
	if err != nil {
		return err
	}
	if !passedRequest.ReturnResponseBody {
		defer response.Body.Close()
	}

	return connection.populateResponse(response, passedRequest.ReturnResponseBody, passedResponse)
}

func (connection *connection) ConnectToEventStream(passedRequest Request) (*sse.EventSource, error) {
	return sse.Connect(connection.httpClient, time.Second, func() *http.Request {
		request, reqErr := connection.createHTTPRequest(passedRequest)
		if reqErr != nil {
			panic("unexpected error creating request: " + reqErr.Error())
		}

		return request
	})
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

func (connection *connection) getBody(passedRequest Request) *bytes.Buffer {
	if passedRequest.Header != nil && passedRequest.Body != nil {
		if _, ok := passedRequest.Header["Content-Type"]; !ok {
			panic("You must pass a 'Content-Type' Header with a body")
		}
		return passedRequest.Body
	}

	return &bytes.Buffer{}
}

func (connection *connection) populateResponse(response *http.Response, returnResponseBody bool, passedResponse *Response) error {
	if response.StatusCode == http.StatusNotFound {
		return ResourceNotFoundError{}
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

	if passedResponse.Headers != nil {
		for k, v := range response.Header {
			(*passedResponse.Headers)[k] = v
		}
	}

	return nil
}
