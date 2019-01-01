package healthcheck

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
)

func doRequest(ctx context.Context, method, url string, body io.Reader, respObj interface{}) error {
	var (
		req  *http.Request
		resp *http.Response
		err  error
	)

	req, err = http.NewRequest(method, url, body)
	if err != nil {
		return errors.Wrapf(err,
			"couldn't create request obj")
	}

	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrapf(err,
			"failed to send http request")
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseMessage, _ := ioutil.ReadAll(resp.Body)

		return errors.Errorf("non-success status code %d - %s",
			resp.StatusCode, string(responseMessage))
	}

	if respObj != nil {
		decoder := json.NewDecoder(resp.Body)
		err = decoder.Decode(respObj)
		if err != nil {
			return errors.Wrapf(err,
				"failed to decode response into provided interface")
		}
	}

	return nil
}
