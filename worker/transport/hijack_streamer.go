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

	"code.cloudfoundry.org/garden"
	"github.com/concourse/atc/db"
	"github.com/concourse/retryhttp"
	"github.com/tedsuo/rata"
)

//go:generate counterfeiter . TransportDB
type TransportDB interface {
	GetWorker(name string) (db.Worker, bool, error)
}

//go:generate counterfeiter . ReadCloser
type ReadCloser interface {
	Read(p []byte) (n int, err error)
	Close() error
}

//go:generate counterfeiter . RequestGenerator
type RequestGenerator interface {
	CreateRequest(name string, params rata.Params, body io.Reader) (*http.Request, error)
}

// instead of httpClient defined in default Garden HijackStreamer
type WorkerHijackStreamer struct {
	HttpClient       *http.Client
	HijackableClient retryhttp.HijackableClient
	Req              RequestGenerator
}

func (h *WorkerHijackStreamer) Stream(handler string, body io.Reader, params rata.Params, query url.Values, contentType string) (io.ReadCloser, error) {
	request, err := h.Req.CreateRequest(handler, params, body)
	if err != nil {
		return nil, err
	}

	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}

	if query != nil {
		request.URL.RawQuery = query.Encode()
	}

	httpResp, err := h.HttpClient.Do(request)
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

func (h *WorkerHijackStreamer) Hijack(handler string, body io.Reader, params rata.Params, query url.Values, contentType string) (net.Conn, *bufio.Reader, error) {
	request, err := h.Req.CreateRequest(handler, params, body)
	if err != nil {
		return nil, nil, err
	}

	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}

	if query != nil {
		request.URL.RawQuery = query.Encode()
	}

	httpResp, hijackCloser, err := h.HijackableClient.Do(request)
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
