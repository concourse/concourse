package commands

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"

	"github.com/inconshreveable/go-update"
	"github.com/tedsuo/rata"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
)

type SyncCommand struct{}

var syncCommand SyncCommand

func init() {
	sync, err := Parser.AddCommand(
		"sync",
		"Download and replace the current fly from the target",
		"",
		&syncCommand,
	)
	if err != nil {
		panic(err)
	}

	sync.Aliases = []string{"s"}
}

func (command *SyncCommand) Execute(args []string) error {
	target, err := rc.SelectTarget(globalOptions.Target, globalOptions.Insecure)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	reqGenerator := rata.NewRequestGenerator(target.URL(), atc.Routes)

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

	tlsConfig := &tls.Config{InsecureSkipVerify: target.Insecure}

	transport := &http.Transport{TLSClientConfig: tlsConfig}

	client := &http.Client{Transport: transport}

	response, err := client.Do(request)
	if err != nil {
		log.Fatalln(err)
	}

	if response.StatusCode != http.StatusOK {
		failf("bad response: %s", response.Status)
	}

	fmt.Printf("downloading fly from %s... ", request.URL.Host)

	err = update.Apply(response.Body, update.Options{})
	if err != nil {
		failf("update failed: %s", err)
	}

	fmt.Println("update successful!")
	return nil
}
