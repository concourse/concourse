package commands

import (
	"fmt"
	"net/url"
	"runtime"

	"github.com/codegangsta/cli"
	"github.com/inconshreveable/go-update"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc"
)

func Sync(c *cli.Context) {
	atcURL := c.GlobalString("atcURL")
	reqGenerator := rata.NewRequestGenerator(atcURL, atc.Routes)

	request, err := reqGenerator.CreateRequest(
		atc.DownloadCLI, rata.Params{}, nil,
	)
	if err != nil {
		fmt.Printf("Building request failed: %v\n", err)
	}

	request.URL.RawQuery = url.Values{
		"arch":     []string{runtime.GOARCH},
		"platform": []string{runtime.GOOS},
	}.Encode()

	err, errRecover := update.New().FromUrl(request.URL.String())
	if err != nil {
		fmt.Printf("Update failed: %v\n", err)
		if errRecover != nil {
			fmt.Printf("Failed to recover previous executable: %v!\n", errRecover)
			fmt.Printf("Things are probably in a bad state on your machine now.\n")
		}

		return
	}

	fmt.Println("Update successful!")
}
