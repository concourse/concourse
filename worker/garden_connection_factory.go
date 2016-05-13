package worker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	gconn "github.com/cloudfoundry-incubator/garden/client/connection"
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
	BuildConnection(HijackStreamer) gconn.Connection
	CreateRetryableHttpClient() http.Client
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

func NewGardenConnectionFactory(
	db GardenConnectionFactoryDB,
	logger lager.Logger,
	workerName string,
	address string,
) GardenConnectionFactory {
	return &gardenConnectionFactory{
		db:         db,
		logger:     logger,
		workerName: workerName,
		address:    address,
	}
}

func (gcf *gardenConnectionFactory) BuildConnection(hijacker HijackStreamer) gconn.Connection {
	return gconn.NewWithHijacker(hijacker, gcf.logger)
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

	request.URL.Host = roundTripper.cachedHost
	response, err := roundTripper.httpRoundTripper.RoundTrip(request)
	if err != nil {
		roundTripper.cachedHost = ""
	}
	return response, err
}

// WorkerHijackStreamer implements Stream that is using our httpClient,
// instead of httpClient defined in default Garden HijackStreamer
type WorkerHijackStreamer struct {
	delegate   gconn.HijackStreamer
	httpClient http.Client
	req        *rata.RequestGenerator
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
	return h.delegate.Hijack(handler, body, params, query, contentType)
}
