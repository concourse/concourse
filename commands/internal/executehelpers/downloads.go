package executehelpers

import (
	"fmt"
	"net/http"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/deprecated"
	"github.com/tedsuo/rata"
)

func Download(output Output, atcRequester *deprecated.AtcRequester) {
	path := output.Path
	pipe := output.Pipe

	downloadBits, err := atcRequester.CreateRequest(
		atc.ReadPipe,
		rata.Params{"pipe_id": pipe.ID},
		nil,
	)
	if err != nil {
		panic(err)
	}

	response, err := atcRequester.HttpClient.Do(downloadBits)
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
