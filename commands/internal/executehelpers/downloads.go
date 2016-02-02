package executehelpers

import (
	"fmt"
	"net/http"
	"os"

	"github.com/concourse/go-concourse/concourse"
)

func Download(client concourse.Client, output Output) {
	path := output.Path
	pipe := output.Pipe

	response, err := client.HTTPClient().Get(pipe.ReadURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "download request failed:", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, badResponseError("downloading bits", response))
		panic("unexpected-response-code")
	}

	err = os.MkdirAll(path, 0755)
	if err != nil {
		panic(err)
	}

	err = tarStreamTo(path, response.Body)
	if err != nil {
		panic(err)
	}
}
