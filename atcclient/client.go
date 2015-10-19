package atcclient

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
	"github.com/tedsuo/rata"
)

//go:generate counterfeiter . Client
type Client interface {
	Send(request Request, response Response) error
}

type Request struct {
	RequestName string
	Params      map[string]string
	Queries     map[string]string
	Headers     map[string][]string
	Body        interface{}
}

type Response struct {
	Result  interface{}
	Headers *map[string][]string
}

type UnexpectedResponseError struct {
	error
	StatusCode int
	Status     string
	Body       string
}

func (e UnexpectedResponseError) Error() string {
	return fmt.Sprintf("Unexpected Response\nStatus: %s\nBody:\n%s", e.Status, e.Body)
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

func (client *AtcClient) Send(passedRequest Request, passedResponse Response) error {
	req, err := client.createHttpRequest(passedRequest)

	response, err := client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return client.populateResponse(response, passedResponse)
}

func (client *AtcClient) createHttpRequest(passedRequest Request) (*http.Request, error) {
	buffer := &bytes.Buffer{}
	if passedRequest.Body != nil {
		err := json.NewEncoder(buffer).Encode(passedRequest.Body)
		if err != nil {
			return nil, err
		}
	}

	req, err := client.requestGenerator.CreateRequest(
		passedRequest.RequestName,
		passedRequest.Params,
		buffer,
	)
	if err != nil {
		return nil, err
	}

	values := url.Values{}
	for k, v := range passedRequest.Queries {
		values[k] = []string{v}
	}
	req.URL.RawQuery = values.Encode()

	if passedRequest.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

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

func (client *AtcClient) populateResponse(response *http.Response, passedResponse Response) error {
	if response.StatusCode == http.StatusNoContent {
		return nil
	} else if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(response.Body)

		return UnexpectedResponseError{
			StatusCode: response.StatusCode,
			Status:     response.Status,
			Body:       string(body),
		}
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
