package commands

import (
	atcroutes "github.com/concourse/atc/web/routes"
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
	target := returnTarget(globalOptions.Target)
	insecure := globalOptions.Insecure
	asJSON := command.JSON
	pipelineName := command.Pipeline

	apiRequester := newAtcRequester(target, insecure)
	webRequestGenerator := rata.NewRequestGenerator(target, atcroutes.Routes)

	atcConfig := ATCConfig{
		pipelineName:        pipelineName,
		apiRequester:        apiRequester,
		webRequestGenerator: webRequestGenerator,
	}

	atcConfig.Dump(asJSON)
	return nil
}
