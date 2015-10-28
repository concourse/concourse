package commands

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"
	"github.com/concourse/fly/rc"
)

type GetPipelineCommand struct {
	Pipeline string `short:"p" long:"pipeline" required:"true" description:"Get configuration of this pipeline"`
	JSON     bool   `short:"j" long:"json"                     description:"Print config as json instead of yaml"`
}

func (command *GetPipelineCommand) Execute(args []string) error {
	asJSON := command.JSON
	pipelineName := command.Pipeline

	connection, err := rc.TargetConnection(Fly.Target)
	if err != nil {
		log.Fatalln(err)
	}

	client := concourse.NewClient(connection)
	config, _, _, err := client.PipelineConfig(pipelineName)
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
