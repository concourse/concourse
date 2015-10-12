package commands

import (
	"log"

	atcroutes "github.com/concourse/atc/web/routes"
	"github.com/concourse/fly/rc"
	"github.com/tedsuo/rata"
)

type GetConfigCommand struct {
	Pipeline string `short:"p" long:"pipeline" required:"true" description:"Get configuration of this pipeline"`
	JSON     bool   `short:"j" long:"json"                     description:"Print config as json instead of yaml"`
}

var getConfigCommand GetConfigCommand

func init() {
	configure, err := Parser.AddCommand(
		"get-config",
		"Dowload pipeline configuration",
		"",
		&getConfigCommand,
	)
	if err != nil {
		panic(err)
	}

	configure.Aliases = []string{"gc"}
}

func (command *GetConfigCommand) Execute(args []string) error {
	target, err := rc.SelectTarget(globalOptions.Target, globalOptions.Insecure)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	asJSON := command.JSON
	pipelineName := command.Pipeline

	apiRequester := newAtcRequester(target.URL(), target.Insecure)
	webRequestGenerator := rata.NewRequestGenerator(target.URL(), atcroutes.Routes)

	atcConfig := ATCConfig{
		pipelineName:        pipelineName,
		apiRequester:        apiRequester,
		webRequestGenerator: webRequestGenerator,
	}

	atcConfig.Dump(asJSON)
	return nil
}
