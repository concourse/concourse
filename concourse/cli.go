package concourse

import (
	"errors"
	"io"
	"net/url"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse/internal"
)

func (client *client) GetCLIReader(arch, platform string) (io.ReadCloser, error) {
	response := internal.Response{}

	err := client.connection.Send(internal.Request{
		RequestName: atc.DownloadCLI,
		Query: url.Values{
			"arch":     {arch},
			"platform": {platform},
		},
		ReturnResponseBody: true,
	},
		&response,
	)
	if err != nil {
		return nil, err
	}

	readCloser, ok := response.Result.(io.ReadCloser)
	if !ok {
		return nil, errors.New("Unable to get stream from response.")
	}

	return readCloser, nil
}
