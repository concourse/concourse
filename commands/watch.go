package commands

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/codegangsta/cli"
	"github.com/concourse/atc"
	"github.com/concourse/fly/eventstream"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"
)

func Watch(c *cli.Context) {
	target := c.GlobalString("target")
	insecure := c.GlobalBool("insecure")

	atcRequester := newAtcRequester(target, insecure)

	build := getBuild(c, atcRequester.httpClient, atcRequester.RequestGenerator)

	eventSource, err := sse.Connect(atcRequester.httpClient, time.Second, func() *http.Request {
		logOutput, err := atcRequester.CreateRequest(
			atc.BuildEvents,
			rata.Params{"build_id": strconv.Itoa(build.ID)},
			nil,
		)
		if err != nil {
			log.Fatalln(err)
		}

		return logOutput
	})
	if err != nil {
		log.Println("failed to attach to stream:", err)
		os.Exit(1)
	}

	exitCode, err := eventstream.RenderStream(eventSource)
	if err != nil {
		log.Println("failed to render stream:", err)
		os.Exit(1)
	}

	eventSource.Close()

	os.Exit(exitCode)
}
