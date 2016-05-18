package worker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	gconn "github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/cloudfoundry-incubator/garden/routes"
	"github.com/concourse/atc/db"
	"github.com/concourse/retryhttp"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
)

//go:generate counterfeiter . GardenConnectionFactoryDB
type GardenConnectionFactoryDB interface {
	GetWorker(string) (db.SavedWorker, bool, error)
}

//go:generate counterfeiter . GardenConnectionFactory
type GardenConnectionFactory interface {
	BuildConnection() gconn.Connection
	CreateRetryableHttpClient() http.Client
	CreateRetryHijackableClient() retryhttp.HijackableClient
}

//go:generate counterfeiter . HijackStreamer
type HijackStreamer interface {
	Stream(handler string, body io.Reader, params rata.Params, query url.Values, contentType string) (io.ReadCloser, error)
	Hijack(handler string, body io.Reader, params rata.Params, query url.Values, contentType string) (net.Conn, *bufio.Reader, error)
}

type gardenConnectionFactory struct {
	db         GardenConnectionFactoryDB
	streamer   WorkerHijackStreamer
	logger     lager.Logger
	workerName string
	address    string
}

type WorkerLookupRoundTripper struct {
	db               GardenConnectionFactoryDB
	workerName       string
	httpRoundTripper retryhttp.RoundTripper
	cachedHost       string
}

type WorkerLookupHijackableClient struct {
	db               GardenConnectionFactoryDB
	workerName       string
	hijackableClient retryhttp.HijackableClient
	cachedHost       string
}

func NewGardenConnectionFactory(
	db GardenConnectionFactoryDB,
	logger lager.Logger,
	workerName string,
) GardenConnectionFactory {
	return &gardenConnectionFactory{
		db:         db,
		logger:     logger,
		workerName: workerName,
	}
}

func (gcf *gardenConnectionFactory) BuildConnection() gconn.Connection {
	hijackStreamer := WorkerHijackStreamer{
		httpClient: gcf.CreateRetryableHttpClient(),
		// the request generator's address doesn't matter because it's overwritten by the worker lookup clients
		req:              rata.NewRequestGenerator("http://127.0.0.1:8080", routes.Routes),
		hijackableClient: gcf.CreateRetryHijackableClient(),
	}
	return gconn.NewWithHijacker(hijackStreamer, gcf.logger)
}

func (gcf *gardenConnectionFactory) CreateRetryableHttpClient() http.Client {
	retryRoundTripper := retryhttp.RetryRoundTripper{
		Logger:  lager.NewLogger("retryable-http-client"),
		Sleeper: clock.NewClock(),
		RetryPolicy: ExponentialRetryPolicy{
			Timeout: 60 * time.Minute,
		},
		RoundTripper: CreateWorkerLookupRoundTripper(gcf.workerName,
			gcf.db,
			&http.Transport{DisableKeepAlives: true}),
	}

	return http.Client{
		Transport: retryRoundTripper.RoundTripper,
	}
}

func (gcf *gardenConnectionFactory) CreateRetryHijackableClient() retryhttp.HijackableClient {
	return &retryhttp.RetryHijackableClient{
		Logger:  lager.NewLogger("retry-hijackable-client"),
		Sleeper: clock.NewClock(),
		RetryPolicy: ExponentialRetryPolicy{
			Timeout: 60 * time.Minute,
		},
		HijackableClient: CreateWorkerLookupHijackableClient(
			gcf.workerName,
			gcf.db,
			retryhttp.DefaultHijackableClient,
		),
	}
}

func CreateWorkerLookupRoundTripper(workerName string, db GardenConnectionFactoryDB, innerRoundTripper http.RoundTripper) http.RoundTripper {
	return &WorkerLookupRoundTripper{
		httpRoundTripper: innerRoundTripper,
		workerName:       workerName,
		db:               db,
		cachedHost:       "",
	}
}

func (roundTripper *WorkerLookupRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if roundTripper.cachedHost == "" {
		savedWorker, found, err := roundTripper.db.GetWorker(roundTripper.workerName)
		if err != nil {
			return nil, err
		}

		if !found {
			return nil, ErrMissingWorker
		}
		roundTripper.cachedHost = savedWorker.GardenAddr
	}

	updatedURL := *request.URL
	updatedURL.Host = roundTripper.cachedHost

	updatedRequest := *request
	updatedRequest.URL = &updatedURL

	response, err := roundTripper.httpRoundTripper.RoundTrip(&updatedRequest)
	if err != nil {
		roundTripper.cachedHost = ""
	}
	return response, err
}

func CreateWorkerLookupHijackableClient(workerName string, db GardenConnectionFactoryDB, hijackableClient retryhttp.HijackableClient) retryhttp.HijackableClient {
	return &WorkerLookupHijackableClient{
		hijackableClient: hijackableClient,
		workerName:       workerName,
		db:               db,
		cachedHost:       "",
	}
}

func (c *WorkerLookupHijackableClient) Do(request *http.Request) (*http.Response, retryhttp.HijackCloser, error) {
	if c.cachedHost == "" {
		savedWorker, found, err := c.db.GetWorker(c.workerName)
		if err != nil {
			return nil, nil, err
		}

		if !found {
			return nil, nil, ErrMissingWorker
		}
		c.cachedHost = savedWorker.GardenAddr
	}

	updatedURL := *request.URL
	updatedURL.Host = c.cachedHost

	updatedRequest := *request
	updatedRequest.URL = &updatedURL

	response, hijackCloser, err := c.hijackableClient.Do(&updatedRequest)
	if err != nil {
		c.cachedHost = ""
	}
	return response, hijackCloser, err
}

// WorkerHijackStreamer implements Stream that is using our httpClient,
// instead of httpClient defined in default Garden HijackStreamer
type WorkerHijackStreamer struct {
	httpClient       http.Client
	req              *rata.RequestGenerator
	hijackableClient retryhttp.HijackableClient
}

func (h WorkerHijackStreamer) Stream(handler string, body io.Reader, params rata.Params, query url.Values, contentType string) (io.ReadCloser, error) {
	request, err := h.req.CreateRequest(handler, params, body)
	if err != nil {
		return nil, err
	}

	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}

	if query != nil {
		request.URL.RawQuery = query.Encode()
	}

	httpResp, err := h.httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode > 299 {
		defer httpResp.Body.Close()

		var result garden.Error
		err := json.NewDecoder(httpResp.Body).Decode(&result)
		if err != nil {
			return nil, fmt.Errorf("bad response: %s", err)
		}

		return nil, result.Err
	}

	return httpResp.Body, nil
}

func (h WorkerHijackStreamer) Hijack(handler string, body io.Reader, params rata.Params, query url.Values, contentType string) (net.Conn, *bufio.Reader, error) {
	request, err := h.req.CreateRequest(handler, params, body)
	if err != nil {
		return nil, nil, err
	}

	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}

	if query != nil {
		request.URL.RawQuery = query.Encode()
	}

	httpResp, hijackCloser, err := h.hijackableClient.Do(request)
	if err != nil {
		return nil, nil, err
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode > 299 {
		defer hijackCloser.Close()
		defer httpResp.Body.Close()

		errRespBytes, err := ioutil.ReadAll(httpResp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("Backend error: Exit status: %d, error reading response body: %s", httpResp.StatusCode, err)
		}

		return nil, nil, fmt.Errorf("Backend error: Exit status: %d, message: %s", httpResp.StatusCode, errRespBytes)
	}

	hijackedConn, hijackedResponseReader := hijackCloser.Hijack()

	return hijackedConn, hijackedResponseReader, nil
}
