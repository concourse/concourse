package commands

import (
	"errors"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/commands/internal/setpipelinehelpers"
	"github.com/concourse/concourse/fly/commands/internal/templatehelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/go-concourse/concourse"

	"github.com/imdario/mergo"
	"github.com/mgutz/ansi"
)

type SetPipelineCommand struct {
	SkipInteractive  bool `short:"n"  long:"non-interactive"               description:"Skips interactions, uses default values"`
	DisableAnsiColor bool `long:"no-color"               description:"Disable color output"`

	CheckCredentials bool `long:"check-creds"  description:"Validate credential variables against credential manager"`

	PipelineName string       `short:"p"  long:"pipeline"  required:"true"  description:"Pipeline to configure"`
	Config       atc.PathFlag `short:"c"  long:"config"    required:"true"  description:"Pipeline configuration file, \"-\" stands for stdin"`

	Var     []flaghelpers.VariablePairFlag     `short:"v"  long:"var"       value-name:"[NAME=STRING]"  description:"Specify a string value to set for a variable in the pipeline"`
	YAMLVar []flaghelpers.YAMLVariablePairFlag `short:"y"  long:"yaml-var"  value-name:"[NAME=YAML]"    description:"Specify a YAML value to set for a variable in the pipeline"`

	VarsFrom []atc.PathFlag `short:"l"  long:"load-vars-from"  description:"Variable flag that can be used for filling in template values in configuration from a YAML file"`

	InstanceVars []flaghelpers.InstanceVarPairFlag `short:"i"  long:"instance-var"  hidden:"true"  value-name:"[NAME=STRING]"  description:"Specify a string value to set for a variable in the pipeline and an instanced pipeline identifier"`

	Team string `long:"team"              description:"Name of the team to which the pipeline belongs, if different from the target default"`
}

func (command *SetPipelineCommand) Validate() ([]concourse.ConfigWarning, error) {
	var warnings []concourse.ConfigWarning
	var err error
	if strings.Contains(command.PipelineName, "/") {
		err = errors.New("pipeline name cannot contain '/'")
	}
	if command.Team != "" {
		if warning := atc.ValidateIdentifier(command.Team, "team"); warning != nil {
			warnings = append(warnings, concourse.ConfigWarning{
				Type:    warning.Type,
				Message: warning.Message,
			})
		}
	}
	return warnings, err
}

func (command *SetPipelineCommand) Execute(args []string) error {
	warnings, err := command.Validate()
	if err != nil {
		return err
	}
	configPath := command.Config
	templateVariablesFiles := command.VarsFrom
	pipelineName := command.PipelineName

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	var team concourse.Team

	if command.Team != "" {
		team, err = target.FindTeam(command.Team)
		if err != nil {
			return err
		}
	} else {
		team = target.Team()
	}

	ansi.DisableColors(command.DisableAnsiColor)

	var instanceVars atc.InstanceVars
	if command.InstanceVars != nil {
		dot := &atc.DotNotation{}
		for _, iv := range command.InstanceVars {
			err := mergo.Merge(dot, iv.Value)
			if err != nil {
				return err
			}
		}
		instanceVars, err = dot.Expand()
		if err != nil {
			return err
		}
	}

	atcConfig := setpipelinehelpers.ATCConfig{
		Team: team,
		PipelineRef: atc.PipelineRef{
			Name:         pipelineName,
			InstanceVars: instanceVars,
		},
		TargetName:       Fly.Target,
		Target:           target.Client().URL(),
		SkipInteraction:  command.SkipInteractive || command.Config.FromStdin(),
		CheckCredentials: command.CheckCredentials,
		CommandWarnings:  warnings,
	}

	yamlTemplateWithParams := templatehelpers.NewYamlTemplateWithParams(configPath, templateVariablesFiles, command.Var, command.YAMLVar, instanceVars)
	return atcConfig.Set(yamlTemplateWithParams)
}
