package reaper

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/retryhttp"
	"github.com/concourse/worker/reaper/api"
	"github.com/tedsuo/rata"
)

var ErrUnreachableGardenServer = errors.New("Unable to reach garden")

type client struct {
	requestGenerator *rata.RequestGenerator
	httpClient       *http.Client
	logger           lager.Logger
}

// NewClient provides a new ReaperClient based on provided URL
func NewClient(apiURL string, logger lager.Logger) ReaperClient {
	return &client{
		requestGenerator: rata.NewRequestGenerator(apiURL, api.Routes),
		httpClient:       http.DefaultClient,
		logger:           logger,
	}
}

func New(apiURL string, nestedRoundTripper http.RoundTripper, logger lager.Logger) ReaperClient {
	return &client{
		requestGenerator: rata.NewRequestGenerator(apiURL, api.Routes),
		httpClient: &http.Client{
			Transport: &retryhttp.RetryRoundTripper{
				Logger:         logger.Session("retry-round-tripper"),
				BackOffFactory: retryhttp.NewExponentialBackOffFactory(60 * time.Minute),
				RoundTripper:   nestedRoundTripper,
				Retryer:        &retryhttp.DefaultRetryer{},
			},
		},
	}
}

// NewWithHttpClient provides a ReaperClient based on provided URL and http.Client
func NewWithHttpClient(apiURL string, logger lager.Logger, httpClient *http.Client) ReaperClient {
	return &client{
		requestGenerator: rata.NewRequestGenerator(apiURL, api.Routes),
		httpClient:       httpClient,
		logger:           logger,
	}
}

func (c *client) Ping() error {
	request, _ := c.requestGenerator.CreateRequest(api.Ping, nil, nil)
	res, err := c.httpClient.Do(request)
	if err != nil {
		c.logger.Error("failed-to-connect-to-reaper-server", err)
		return err
	}
	if res.StatusCode != http.StatusOK {
		return ErrUnreachableGardenServer
	}
	return nil
}

func (c *client) DestroyContainers(handles []string) error {
	requestBody, err := json.Marshal(handles)
	if err != nil {
		return err
	}

	request, _ := c.requestGenerator.CreateRequest(api.DestroyContainers, nil, bytes.NewReader(requestBody))
	response, err := c.httpClient.Do(request)
	if err != nil {
		c.logger.Error("failed-to-connect-to-reaper-server", err)
		return err
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return errors.New("failed-to-destroy-containers")
	}

	return nil
}
