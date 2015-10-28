package atcclient

import (
	"errors"
	"io"

	"github.com/concourse/atc"
)

func (client *client) GetCLIReader(arch, platform string) (io.ReadCloser, error) {
	response := Response{}

	err := client.connection.Send(Request{
		RequestName: atc.DownloadCLI,
		Queries: map[string]string{
			"arch":     arch,
			"platform": platform,
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
