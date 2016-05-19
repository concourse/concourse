package transport

// WorkerHijackStreamer implements Stream that is using our httpClient,
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

type ErrMissingWorker struct {
	WorkerName string
}

func (e ErrMissingWorker) Error() string {
	return fmt.Sprintf("worker %s not found in database while retrying http request", e.WorkerName)
}

//go:generate counterfeiter . TransportDB
type TransportDB interface {
	GetWorker(string) (db.SavedWorker, bool, error)
}

// instead of httpClient defined in default Garden HijackStreamer
type hijackStreamer struct {
	httpClient       http.Client
	hijackableClient retryhttp.HijackableClient
	req              *rata.RequestGenerator
}

func NewHijackStreamer(logger lager.Logger, workerName string, db TransportDB) gconn.HijackStreamer {
	retryPolicy := ExponentialRetryPolicy{
		Timeout: 60 * time.Minute,
	}

	httpClient := http.Client{
		Transport: &retryhttp.RetryRoundTripper{
			Logger:       logger.Session("retryable-http-client"),
			Sleeper:      clock.NewClock(),
			RetryPolicy:  retryPolicy,
			RoundTripper: NewRoundTripper(workerName, db, &http.Transport{DisableKeepAlives: true}),
		},
	}

	hijackableClient := &retryhttp.RetryHijackableClient{
		Logger:           logger.Session("retry-hijackable-client"),
		Sleeper:          clock.NewClock(),
		RetryPolicy:      retryPolicy,
		HijackableClient: NewHijackableClient(workerName, db, retryhttp.DefaultHijackableClient),
	}

	return hijackStreamer{
		httpClient:       httpClient,
		hijackableClient: hijackableClient,
		req:              rata.NewRequestGenerator("http://127.0.0.1:8080", routes.Routes),
	}
}

func (h hijackStreamer) Stream(handler string, body io.Reader, params rata.Params, query url.Values, contentType string) (io.ReadCloser, error) {
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

func (h hijackStreamer) Hijack(handler string, body io.Reader, params rata.Params, query url.Values, contentType string) (net.Conn, *bufio.Reader, error) {
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
