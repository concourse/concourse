package client

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/reaper"
	"github.com/tedsuo/rata"
)

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
		requestGenerator: rata.NewRequestGenerator(apiURL, reaper.Routes),
		httpClient:       http.DefaultClient,
		logger:           logger,
	}
}

func (c *client) DestroyContainers(handles []string) error {

	request, _ := c.requestGenerator.CreateRequest(reaper.DestroyContainers, nil, nil)
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
