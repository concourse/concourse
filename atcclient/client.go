package atcclient

import (
	"crypto/tls"
	"encoding/json"
	"errors"
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

	response, err := client.httpClient.Do(req)
	defer response.Body.Close()
	if err != nil {
		return err
	}

	err = json.NewDecoder(response.Body).Decode(result)
	if err != nil {
		return err
	}
	return nil
}
