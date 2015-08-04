package commands

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"

	"github.com/codegangsta/cli"
	"github.com/inconshreveable/go-update"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc"
)

func Sync(c *cli.Context) {
	target := returnTarget(c.GlobalString("target"))
	insecure := c.GlobalBool("insecure")
	reqGenerator := rata.NewRequestGenerator(target, atc.Routes)

	request, err := reqGenerator.CreateRequest(
		atc.DownloadCLI, rata.Params{}, nil,
	)
	if err != nil {
		fmt.Printf("building request failed: %v\n", err)
		os.Exit(1)
	}

	request.URL.RawQuery = url.Values{
		"arch":     []string{runtime.GOARCH},
		"platform": []string{runtime.GOOS},
	}.Encode()

	tlsConfig := &tls.Config{InsecureSkipVerify: insecure}

	transport := &http.Transport{TLSClientConfig: tlsConfig}

	client := &http.Client{Transport: transport}

	updateCustom := &update.Update{HTTPClient: client}

	fmt.Printf("downloading fly from %s... ", request.URL.Host)

	err, errRecover := updateCustom.FromUrl(request.URL.String())
	if err != nil {
		fmt.Printf("update failed: %v\n", err)
		if errRecover != nil {
			fmt.Printf("failed to recover previous executable: %v!\n", errRecover)
			fmt.Printf("things are probably in a bad state on your machine now.\n")
		}

		return
	}

	fmt.Println("update successful!")
}
