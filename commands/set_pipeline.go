package commands

import (
	"log"

	"github.com/concourse/atc/web"
	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/template"
	"github.com/tedsuo/rata"
)

type SetPipelineCommand struct {
	Pipeline string             `short:"p"  long:"pipeline" required:"true"      description:"Pipeline to configure"`
	Config   PathFlag           `short:"c"  long:"config"                        description:"Pipeline configuration file"`
	Var      []VariablePairFlag `short:"v"  long:"var" value-name:"[SECRET=KEY]" description:"Variable flag that can be used for filling in template values in configuration"`
	VarsFrom []PathFlag         `short:"l"  long:"load-vars-from"                description:"Variable flag that can be used for filling in template values in configuration from a YAML file"`
	Paused   string             `long:"paused"         value-name:"[true/false]" description:"Should the pipeline start out as paused or unpaused"`
}

var setPipelineCommand SetPipelineCommand

func init() {
	configure, err := Parser.AddCommand(
		"set-pipeline",
		"Update pipeline configuration",
		"",
		&setPipelineCommand,
	)
	if err != nil {
		panic(err)
	}

	configure.Aliases = []string{"sp"}
}

func (command *SetPipelineCommand) Execute(args []string) error {
	configPath := command.Config
	templateVariablesFiles := command.VarsFrom
	pipelineName := command.Pipeline

	templateVariables := template.Variables{}
	for _, v := range command.Var {
		templateVariables[v.Name] = v.Value
	}

	var paused PipelineAction
	if command.Paused != "" {
		if command.Paused == "true" {
			paused = PausePipeline
		} else if command.Paused == "false" {
			paused = UnpausePipeline
		} else {
			failf(`invalid boolean value "%s" for --paused`, command.Paused)
		}
	} else {
		paused = DoNotChangePipeline
	}

	target, err := rc.SelectTarget(globalOptions.Target, globalOptions.Insecure)
	if err != nil {
		log.Fatalln(err)
		return nil
	}
	client, err := atcclient.NewClient(*target)
	if err != nil {
		log.Fatalln(err)
	}
	handler := atcclient.NewAtcHandler(client)

	webRequestGenerator := rata.NewRequestGenerator(target.URL(), web.Routes)

	atcConfig := ATCConfig{
		pipelineName:        pipelineName,
		webRequestGenerator: webRequestGenerator,
		handler:             handler,
	}

	atcConfig.Set(paused, configPath, templateVariables, templateVariablesFiles)
	return nil
}

type PipelineAction int

const (
	PausePipeline PipelineAction = iota
	UnpausePipeline
	DoNotChangePipeline
)
