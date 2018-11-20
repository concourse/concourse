package commands

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/commands/internal/templatehelpers"
)

type ValidatePipelineCommand struct {
	Config atc.PathFlag `short:"c" long:"config" required:"true"        description:"Pipeline configuration file"`
	Strict bool         `short:"s" long:"strict"                        description:"Fail on warnings"`
	Output bool         `short:"o" long:"output"                        description:"Output templated pipeline to stdout"`

	Var     []flaghelpers.VariablePairFlag     `short:"v"  long:"var"       value-name:"[NAME=STRING]"  description:"Specify a string value to set for a variable in the pipeline"`
	YAMLVar []flaghelpers.YAMLVariablePairFlag `short:"y"  long:"yaml-var"  value-name:"[NAME=YAML]"    description:"Specify a YAML value to set for a variable in the pipeline"`

	VarsFrom []atc.PathFlag `short:"l"  long:"load-vars-from"  description:"Variable flag that can be used for filling in template values in configuration from a YAML file"`
}

func (command *ValidatePipelineCommand) Execute(args []string) error {
	yamlTemplate := templatehelpers.NewYamlTemplateWithParams(command.Config, command.VarsFrom, command.Var, command.YAMLVar)
	return yamlTemplate.Validate(command.Strict, command.Output)
}
