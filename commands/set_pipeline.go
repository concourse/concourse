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
}

func (command *SetPipelineCommand) Execute(args []string) error {
	configPath := command.Config
	templateVariablesFiles := command.VarsFrom
	pipelineName := command.Pipeline

	templateVariables := template.Variables{}
	for _, v := range command.Var {
		templateVariables[v.Name] = v.Value
	}

	connection, err := rc.TargetConnection(Fly.Target)
	if err != nil {
		log.Fatalln(err)
		return nil
	}
	client := atcclient.NewClient(connection)

	webRequestGenerator := rata.NewRequestGenerator(connection.URL(), web.Routes)

	atcConfig := ATCConfig{
		pipelineName:        pipelineName,
		webRequestGenerator: webRequestGenerator,
		client:              client,
	}

	atcConfig.Set(configPath, templateVariables, templateVariablesFiles)
	return nil
}
