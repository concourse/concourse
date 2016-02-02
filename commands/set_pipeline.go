package commands

import (
	"log"

	"github.com/concourse/atc/web"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/commands/internal/setpipelinehelpers"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/template"
	"github.com/tedsuo/rata"
)

type SetPipelineCommand struct {
	Pipeline        string                         `short:"p"  long:"pipeline" required:"true"      description:"Pipeline to configure"`
	Config          flaghelpers.PathFlag           `short:"c"  long:"config" required:"true"        description:"Pipeline configuration file"`
	Var             []flaghelpers.VariablePairFlag `short:"v"  long:"var" value-name:"[SECRET=KEY]" description:"Variable flag that can be used for filling in template values in configuration"`
	VarsFrom        []flaghelpers.PathFlag         `short:"l"  long:"load-vars-from"                description:"Variable flag that can be used for filling in template values in configuration from a YAML file"`
	SkipInteractive bool                           `short:"n"  long:"non-interactive"               description:"Skips interactions, uses default values"`
}

func (command *SetPipelineCommand) Execute(args []string) error {
	configPath := command.Config
	templateVariablesFiles := command.VarsFrom
	pipelineName := command.Pipeline

	templateVariables := template.Variables{}
	for _, v := range command.Var {
		templateVariables[v.Name] = v.Value
	}

	client, err := rc.TargetClient(Fly.Target)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	webRequestGenerator := rata.NewRequestGenerator(client.URL(), web.Routes)

	atcConfig := setpipelinehelpers.ATCConfig{
		PipelineName:        pipelineName,
		WebRequestGenerator: webRequestGenerator,
		Client:              client,
		SkipInteraction:     command.SkipInteractive,
	}

	atcConfig.Set(configPath, templateVariables, templateVariablesFiles)
	return nil
}
