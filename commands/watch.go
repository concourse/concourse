package commands

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/fly/eventstream"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"
)

type WatchCommand struct {
	Job   JobFlag `short:"j" long:"job"   value-name:"[PIPELINE/]JOB"   description:"Watches builds of the given job"`
	Build string  `short:"b" long:"build"                               description:"Watches a specific build"`
}

var watchCommand WatchCommand

func init() {
	watch, err := Parser.AddCommand(
		"watch",
		"Stream a build's log",
		"",
		&watchCommand,
	)
	if err != nil {
		panic(err)
	}

	watch.Aliases = []string{"w"}
}
func (command *WatchCommand) Execute(args []string) error {
	target := returnTarget(globalOptions.Target)
	insecure := globalOptions.Insecure

	atcRequester := newAtcRequester(target, insecure)

	build := getBuild(atcRequester.httpClient, atcRequester.RequestGenerator, command.Job.JobName, command.Build, command.Job.PipelineName)

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
	return nil
}
