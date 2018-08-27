package commands

import (
	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/commands/internal/setpipelinehelpers"
	"github.com/concourse/fly/rc"
	"github.com/mgutz/ansi"
)

type SetPipelineCommand struct {
	SkipInteractive  bool `short:"n"  long:"non-interactive"               description:"Skips interactions, uses default values"`
	DisableAnsiColor bool `long:"no-color"               description:"Disable color output"`

	CheckCredentials bool `long:"check-creds"  description:"Validate credential variables against credential manager"`

	Pipeline flaghelpers.PipelineFlag `short:"p"  long:"pipeline"  required:"true"  description:"Pipeline to configure"`
	Config   atc.PathFlag             `short:"c"  long:"config"    required:"true"  description:"Pipeline configuration file"`

	Var     []flaghelpers.VariablePairFlag     `short:"v"  long:"var"       value-name:"[NAME=STRING]"  description:"Specify a string value to set for a variable in the pipeline"`
	YAMLVar []flaghelpers.YAMLVariablePairFlag `short:"y"  long:"yaml-var"  value-name:"[NAME=YAML]"    description:"Specify a YAML value to set for a variable in the pipeline"`

	VarsFrom []atc.PathFlag `short:"l"  long:"load-vars-from"  description:"Variable flag that can be used for filling in template values in configuration from a YAML file"`
}

func (command *SetPipelineCommand) Validate() error {
	return command.Pipeline.Validate()
}

func (command *SetPipelineCommand) Execute(args []string) error {
	err := command.Validate()
	if err != nil {
		return err
	}
	configPath := command.Config
	templateVariablesFiles := command.VarsFrom
	pipelineName := string(command.Pipeline)

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	ansi.DisableColors(command.DisableAnsiColor)

	atcConfig := setpipelinehelpers.ATCConfig{
		Team:             target.Team(),
		PipelineName:     pipelineName,
		Target:           target.Client().URL(),
		SkipInteraction:  command.SkipInteractive,
		CheckCredentials: command.CheckCredentials,
	}

	return atcConfig.Set(configPath, command.Var, command.YAMLVar, templateVariablesFiles)
}
