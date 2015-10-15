package atcclient

import (
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
	MakeRequest(result interface{}, requestName string, params map[string]string, queries map[string]string) error
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

func (client *AtcClient) MakeRequest(result interface{}, requestName string, params map[string]string, queries map[string]string) error {
	req, err := client.requestGenerator.CreateRequest(
		requestName,
		params,
		nil,
	)
	if err != nil {
		return err
	}

	values := url.Values{}
	for k, v := range queries {
		values[k] = []string{v}
	}
	req.URL.RawQuery = values.Encode()

	if client.target.Username != "" {
		req.SetBasicAuth(client.target.Username, client.target.Password)
	}

	response, err := client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

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

	err = json.NewDecoder(response.Body).Decode(result)
	if err != nil {
		return err
	}
	return nil
}
