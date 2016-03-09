package commands

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v2"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
)

type GetPipelineCommand struct {
	Pipeline string `short:"p" long:"pipeline" required:"true" description:"Get configuration of this pipeline"`
	JSON     bool   `short:"j" long:"json"                     description:"Print config as json instead of yaml"`
}

func (command *GetPipelineCommand) Execute(args []string) error {
	asJSON := command.JSON
	pipelineName := command.Pipeline

	client, err := rc.TargetClient(Fly.Target)
	if err != nil {
		return err
	}
	err = rc.ValidateClient(client, Fly.Target)
	if err != nil {
		return err
	}

	config, _, _, err := client.PipelineConfig(pipelineName)
	if err != nil {
		return err
	}

	return dump(config, asJSON)

	return nil
}

func dump(config atc.Config, asJSON bool) error {
	var payload []byte
	var err error
	if asJSON {
		payload, err = json.Marshal(config)
	} else {
		payload, err = yaml.Marshal(config)
	}
	if err != nil {
		return err
	}

	_, err = fmt.Printf("%s", payload)
	if err != nil {
		return err
	}

	return nil
}
