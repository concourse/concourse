package atcclient

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"
)

//go:generate counterfeiter . Client

type Client interface {
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
	target           rc.TargetProps
	requestGenerator *rata.RequestGenerator
	httpClient       *http.Client
}

func NewClient(target rc.TargetProps) (Client, error) {
	if target.API == "" {
		return nil, errors.New("API is blank")
	}

	tlsClientConfig := &tls.Config{InsecureSkipVerify: target.Insecure}
	client := AtcClient{
		target:           target,
		requestGenerator: rata.NewRequestGenerator(target.API, atc.Routes),
		httpClient:       &http.Client{Transport: &http.Transport{TLSClientConfig: tlsClientConfig}},
	}

	return &client, nil
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

	if client.target.Username != "" {
		req.SetBasicAuth(client.target.Username, client.target.Password)
	}

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
	switch {
	case response.StatusCode == http.StatusNoContent:
		return nil
	case response.StatusCode == http.StatusNotFound:
		return ResourceNotFoundError{}
	case response.StatusCode == http.StatusCreated:
		passedResponse.Created = true
	case response.StatusCode < 200, response.StatusCode >= 300:
		body, _ := ioutil.ReadAll(response.Body)

		return UnexpectedResponseError{
			StatusCode: response.StatusCode,
			Status:     response.Status,
			Body:       string(body),
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

	if passedResponse.Headers != nil {
		for k, v := range response.Header {
			(*passedResponse.Headers)[k] = v
		}
	}

	return nil
}
