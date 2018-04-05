package client

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/worker/reaper"
	"github.com/concourse/worker/reaper/api"
	"github.com/tedsuo/rata"
)

var ErrUnreachableReaperServer = errors.New("Unable to reach garden")

type Client interface {
	reaper.Client
}

type client struct {
	requestGenerator *rata.RequestGenerator
	httpClient       *http.Client
	logger           lager.Logger
}

func NewClient(apiURL string, logger lager.Logger) Client {
	return &client{
		requestGenerator: rata.NewRequestGenerator(apiURL, api.Routes),
		httpClient:       http.DefaultClient,
		logger:           logger,
	}
}

func NewWithHttpClient(apiURL string, logger lager.Logger, httpClient *http.Client) Client {
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
		return err
	}
	if res.StatusCode != http.StatusOK {
		return ErrUnreachableReaperServer
	}
	return nil
}

func (c *client) DestroyContainers(handles []string) error {

	request, _ := c.requestGenerator.CreateRequest(api.DestroyContainers, nil, nil)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return errors.New("failed-to-destroy-containers")
	}

	return nil
}
