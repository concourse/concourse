package curler

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
)

type Curler struct {
	httpClient   *http.Client
	concourseURL string
}

func New(httpClient *http.Client, concourseURL string) Curler {
	return Curler{
		httpClient:   httpClient,
		concourseURL: concourseURL,
	}
}

func (c Curler) It(apiPath string) (body []byte, respHeaders []byte, err error) {
	url, err := url.Parse(c.concourseURL)
	if err != nil {
		return
	}

	url.Path = path.Join(url.Path, apiPath)
	req, err := http.NewRequest("GET", url.String(), nil)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return
	}

	if resp.StatusCode != 200 {
		err = errors.New(resp.Status)
		return
	}

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	respHeaders, err = httputil.DumpResponse(resp, false)
	return
}
