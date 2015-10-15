package commands

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/concourse/atc"
	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"
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
	asJSON := command.JSON
	pipelineName := command.Pipeline

	target, err := rc.SelectTarget(globalOptions.Target, globalOptions.Insecure)
	if err != nil {
		log.Fatalln(err)
	}

	client, err := atcclient.NewClient(*target)
	if err != nil {
		log.Fatalln(err)
	}
	handler := atcclient.NewAtcHandler(client)
	config, err := handler.PipelineConfig(pipelineName)
	if err != nil {
		log.Fatalln(err)
	}

	dump(config, asJSON)
	return nil
}

func dump(config atc.Config, asJSON bool) {
	var payload []byte
	var err error
	if asJSON {
		payload, err = json.Marshal(config)
	} else {
		payload, err = yaml.Marshal(config)
	}

	if err != nil {
		log.Println("failed to marshal config to YAML:", err)
		os.Exit(1)
	}

	fmt.Printf("%s", payload)
}
