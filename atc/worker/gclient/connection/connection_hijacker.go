package connection

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/routes"
	"github.com/tedsuo/rata"
)

// IMPORTANT NOTE: We don't compile this in because we actually use transport.WorkerHijackStreamer
// most of this folder was just copied in from garden/client as a temp workaround to adding ctx to Hijack()
// we needed connection.go in garden/client package and were forced to import the rest of this stuff to make
// connection_suite_test.go to pass

type DialerFunc func(network, address string) (net.Conn, error)

type hijackable struct {
	req               *rata.RequestGenerator
	noKeepaliveClient *http.Client
	dialer            DialerFunc
}

func NewHijackStreamer(network, address string) HijackStreamer {
	return NewHijackStreamerWithDialer(func(string, string) (net.Conn, error) {
		return net.DialTimeout(network, address, 2*time.Second)
	})
}

func NewHijackStreamerWithDialer(dialFunc DialerFunc) HijackStreamer {
	return &hijackable{
		req:    rata.NewRequestGenerator("http://api", routes.Routes),
		dialer: dialFunc,
		noKeepaliveClient: &http.Client{
			Transport: &http.Transport{
				Dial:              dialFunc,
				DisableKeepAlives: true,
			},
		},
	}
}

func (h *hijackable) Hijack(ctx context.Context, handler string, body io.Reader, params rata.Params, query url.Values, contentType string) (net.Conn, *bufio.Reader, error) {
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

	request = request.WithContext(ctx)

	conn, err := h.dialer("tcp", "api") // net/addr don't matter here
	if err != nil {
		return nil, nil, err
	}

	client := httputil.NewClientConn(conn, nil)

	httpResp, err := client.Do(request)
	if err != nil {
		return nil, nil, err
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode > 299 {
		defer httpResp.Body.Close()

		errRespBytes, err := ioutil.ReadAll(httpResp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("Backend error: Exit status: %d, Body: %s, error reading response body: %s", httpResp.StatusCode, string(errRespBytes), err)
		}

		var result garden.Error
		err = json.Unmarshal(errRespBytes, &result)
		if err != nil {
			return nil, nil, fmt.Errorf("Backend error: Exit status: %d, Body: %s, error reading response body: %s", httpResp.StatusCode, string(errRespBytes), err)
		}

		return nil, nil, result.Err
	}

	hijackedConn, hijackedResponseReader := client.Hijack()

	return hijackedConn, hijackedResponseReader, nil
}

func (c *hijackable) Stream(handler string, body io.Reader, params rata.Params, query url.Values, contentType string) (io.ReadCloser, error) {
	request, err := c.req.CreateRequest(handler, params, body)
	if err != nil {
		return nil, err
	}

	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}

	if query != nil {
		request.URL.RawQuery = query.Encode()
	}

	httpResp, err := c.noKeepaliveClient.Do(request)
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
