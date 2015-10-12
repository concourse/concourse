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

	updateCustom := &update.Update{HTTPClient: client}

	fmt.Printf("downloading fly from %s... ", request.URL.Host)

	err, errRecover := updateCustom.FromUrl(request.URL.String())
	if err != nil {
		fmt.Printf("update failed: %v\n", err)
		if errRecover != nil {
			fmt.Printf("failed to recover previous executable: %v!\n", errRecover)
			fmt.Printf("things are probably in a bad state on your machine now.\n")
		}

		return err
	}

	fmt.Println("update successful!")
	return nil
}
