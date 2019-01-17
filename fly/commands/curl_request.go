package commands

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

type CurlRequest struct {
	Host    string
	Path    string
	Method  string
	Headers []string
	Body    string
}

func (r CurlRequest) CreateHttpRequest() (*http.Request, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}

	url, err := url.Parse(r.Host)
	if err != nil {
		return nil, err
	}

	bodyReader, err := r.createBodyReader()
	if err != nil {
		return nil, err
	}

	url.Path = path.Join(url.Path, r.Path)
	req, err := http.NewRequest(r.Method, url.String(), bodyReader)
	if err != nil {
		return nil, err
	}

	for _, h := range r.Headers {
		parts := strings.SplitN(h, ":", 2)
		req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	}

	return req, nil
}

func (r CurlRequest) createBodyReader() (io.Reader, error) {
	if strings.HasPrefix(r.Body, "@") {
		path := r.Body[1:]
		return os.Open(strings.TrimSpace(path))
	}
	return strings.NewReader(r.Body), nil
}

func (r CurlRequest) validate() error {
	for _, h := range r.Headers {
		if !strings.Contains(h, ":") {
			return invalidHeaderError(h)
		}
	}
	return nil
}

func invalidHeaderError(value string) error {
	return fmt.Errorf("invalid header: %s", value)
}
