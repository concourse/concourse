package commands

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
	"github.com/tedsuo/rata"
)

type DestroyPipelineCommand struct {
	Pipeline string `short:"p"  long:"pipeline" required:"true" description:"Pipeline to destroy"`
}

var destroyPipelineCommand DestroyPipelineCommand

func init() {
	destroyPipeline, err := Parser.AddCommand(
		"destroy-pipeline",
		"destroy a pipeline",
		"",
		&destroyPipelineCommand,
	)
	if err != nil {
		panic(err)
	}

	destroyPipeline.Aliases = []string{"d"}
}

func (command *DestroyPipelineCommand) Execute(args []string) error {
	target, err := rc.SelectTarget(globalOptions.Target, globalOptions.Insecure)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	pipelineName := command.Pipeline

	fmt.Printf("!!! this will remove all data for pipeline `%s`", pipelineName)
	fmt.Println("\n")

	if !askToConfirm("are you sure?") {
		fmt.Println("bailing out")
		os.Exit(1)
	}

	atcRequester := newAtcRequester(target.URL(), target.Insecure)

	deletePipeline, err := atcRequester.CreateRequest(
		atc.DeletePipeline,
		rata.Params{"pipeline_name": pipelineName},
		nil,
	)
	if err != nil {
		log.Fatalln(err)
	}

	resp, err := atcRequester.httpClient.Do(deletePipeline)
	if err != nil {
		log.Println("failed to get config:", err, resp)
		os.Exit(1)
	}

	switch resp.StatusCode {
	case http.StatusInternalServerError:
		fmt.Fprintln(os.Stderr, "unexpected server error")
		os.Exit(1)
	case http.StatusNotFound:
		fmt.Fprintf(os.Stderr, "`%s` does not exist\n", pipelineName)
		os.Exit(1)
	case http.StatusNoContent:
		fmt.Printf("`%s` deleted\n", pipelineName)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unexpected response code: %d\n", resp.StatusCode)
		os.Exit(1)
	}

	return nil
}
