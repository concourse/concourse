package reaper

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/worker/reaper/api"
	"github.com/tedsuo/rata"
)

var ErrUnreachableGardenServer = errors.New("Unable to reach garden")

type Client interface {
	ReaperClient
}

type client struct {
	requestGenerator *rata.RequestGenerator
	givenHTTPClient  *http.Client
	logger           lager.Logger
}

// NewClient provides a new ReaperClient based on provided URL
func NewClient(apiURL string, logger lager.Logger) Client {
	return &client{
		requestGenerator: rata.NewRequestGenerator(apiURL, api.Routes),
		givenHTTPClient: &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives:     false,
				ResponseHeaderTimeout: 20 * time.Second,
				MaxIdleConns:          200,
			},
		},
		logger: logger,
	}
}

func (c *client) httpClient(logger lager.Logger) *http.Client {
	if c.givenHTTPClient != nil {
		return c.givenHTTPClient
	}
	return http.DefaultClient
}

func (c *client) ListHandles() ([]string, error) {
	c.logger.Debug("started-listing-containers")
	defer c.logger.Debug("done-listing-containers")

	request, err := c.requestGenerator.CreateRequest(api.List, nil, nil)
	if err != nil {
		c.logger.Error("failed-to-create-request-to-reaper-server", err)
		return nil, err
	}

	response, err := c.httpClient(c.logger).Do(request)
	if err != nil {
		c.logger.Error("failed-to-connect-to-reaper-server", err)
		return nil, err
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		serverErr := fmt.Errorf("received-%d-response", response.StatusCode)
		c.logger.Error("failed-to-list-containers", serverErr, lager.Data{"status-code": response.StatusCode})
		return nil, serverErr
	}

	handles := []string{}
	err = json.NewDecoder(response.Body).Decode(&handles)
	if err != nil {
		c.logger.Error("failed-to-decode-container-handles", err)
		return nil, err
	}

	c.logger.Debug("success-listing-containers")
	return handles, nil
}

func (c *client) Ping() error {
	c.logger.Debug("started-pinging-reaper-server")
	defer c.logger.Debug("done-pinging-reaper-server")

	request, _ := c.requestGenerator.CreateRequest(api.Ping, nil, nil)
	res, err := c.httpClient(c.logger).Do(request)
	if err != nil {
		c.logger.Error("failed-to-connect-to-reaper-server", err)
		return err
	}
	if res.StatusCode != http.StatusOK {
		c.logger.Error("received-non-200-response", ErrUnreachableGardenServer, lager.Data{"status-code": res.StatusCode})
		return ErrUnreachableGardenServer
	}
	c.logger.Debug("success-pinging-server")
	return nil
}

func (c *client) DestroyContainers(handles []string) error {
	c.logger.Debug("started-destroying-containers")
	defer c.logger.Debug("done-destroying-containers")

	buffer := &bytes.Buffer{}
	err := json.NewEncoder(buffer).Encode(handles)
	if err != nil {
		c.logger.Error("failed-to-encode-container-handles", err)
		return err
	}

	request, err := c.requestGenerator.CreateRequest(api.DestroyContainers, nil, buffer)
	if err != nil {
		c.logger.Error("failed-to-create-request-to-reaper-server", err)
		return err
	}

	request.Header.Add("Content-Type", "application/json")
	response, err := c.httpClient(c.logger).Do(request)
	if err != nil {
		c.logger.Error("failed-to-connect-to-reaper-server", err)
		return err
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		serverErr := fmt.Errorf("received-%d-response", response.StatusCode)
		c.logger.Error("failed-to-destroy-containers", serverErr, lager.Data{"status-code": response.StatusCode})
		return serverErr
	}

	c.logger.Debug("success-destroying-containers")
	return nil
}
