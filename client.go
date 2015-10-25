package atcclient

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

//go:generate counterfeiter . Client

type Client interface {
	URL() string
	HTTPClient() *http.Client

	Send(request Request, response *Response) error
	ConnectToEventStream(request Request) (*sse.EventSource, error)
}

type Request struct {
	RequestName        string
	Params             map[string]string
	Queries            map[string]string
	Headers            map[string][]string
	Body               *bytes.Buffer
	ReturnResponseBody bool
}

type Response struct {
	Result  interface{}
	Headers *map[string][]string
	Created bool
}

type AtcClient struct {
	url        string
	httpClient *http.Client

	requestGenerator *rata.RequestGenerator
}

func NewClient(apiURL string, httpClient *http.Client) (Client, error) {
	if apiURL == "" {
		return nil, errors.New("API is blank")
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	apiURL = strings.TrimRight(apiURL, "/")

	return &AtcClient{
		url:        apiURL,
		httpClient: httpClient,

		requestGenerator: rata.NewRequestGenerator(apiURL, atc.Routes),
	}, nil
}

func (client *AtcClient) URL() string {
	return client.url
}

func (client *AtcClient) HTTPClient() *http.Client {
	return client.httpClient
}

func (client *AtcClient) Send(passedRequest Request, passedResponse *Response) error {
	req, err := client.createHTTPRequest(passedRequest)

	response, err := client.httpClient.Do(req)
	if err != nil {
		return err
	}
	if !passedRequest.ReturnResponseBody {
		defer response.Body.Close()
	}

	return client.populateResponse(response, passedRequest.ReturnResponseBody, passedResponse)
}

func (client *AtcClient) ConnectToEventStream(passedRequest Request) (*sse.EventSource, error) {
	return sse.Connect(client.httpClient, time.Second, func() *http.Request {
		request, reqErr := client.createHTTPRequest(passedRequest)
		if reqErr != nil {
			panic("unexpected error creating request: " + reqErr.Error())
		}

		return request
	})
}

func (client *AtcClient) createHTTPRequest(passedRequest Request) (*http.Request, error) {
	body := client.getBody(passedRequest)

	req, err := client.requestGenerator.CreateRequest(
		passedRequest.RequestName,
		passedRequest.Params,
		body,
	)
	if err != nil {
		return nil, err
	}

	values := url.Values{}
	for k, v := range passedRequest.Queries {
		values[k] = []string{v}
	}
	req.URL.RawQuery = values.Encode()

	if passedRequest.Headers != nil {
		for headerID, headerValues := range passedRequest.Headers {
			for _, val := range headerValues {
				req.Header.Add(headerID, val)
			}
		}
	}

	return req, nil
}

func (client *AtcClient) getBody(passedRequest Request) *bytes.Buffer {
	if passedRequest.Headers != nil && passedRequest.Body != nil {
		if _, ok := passedRequest.Headers["Content-Type"]; !ok {
			panic("You must pass a 'Content-Type' Header with a body")
		}
		return passedRequest.Body
	}

	return &bytes.Buffer{}
}

func (client *AtcClient) populateResponse(response *http.Response, returnResponseBody bool, passedResponse *Response) error {
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
