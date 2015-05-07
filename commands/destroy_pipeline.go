package commands

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/codegangsta/cli"
	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
)

func DestroyPipeline(c *cli.Context) {
	target := returnTarget(c.GlobalString("target"))
	insecure := c.GlobalBool("insecure")

	pipelineName := c.Args().First()

	if pipelineName == "" {
		fmt.Fprintln(os.Stderr, "you must specify a pipeline name!")
		os.Exit(1)
	}

	fmt.Printf("!!! this will remove all data for pipeline `%s`", pipelineName)
	fmt.Println("\n")

	if !askToConfirm("are you sure?") {
		fmt.Println("bailing out")
		os.Exit(1)
	}

	atcRequester := newAtcRequester(target, insecure)

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
}
