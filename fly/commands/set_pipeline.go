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
	"github.com/concourse/concourse/vars"

	"github.com/mgutz/ansi"
)

type SetPipelineCommand struct {
	SkipInteractive  bool `short:"n"  long:"non-interactive"               description:"Skips interactions, uses default values"`
	DisableAnsiColor bool `long:"no-color"               description:"Disable color output"`

	CheckCredentials bool `long:"check-creds"  description:"Validate credential variables against credential manager"`

	PipelineName string       `short:"p"  long:"pipeline"  required:"true"  description:"Pipeline to configure"`
	Config       atc.PathFlag `short:"c"  long:"config"    required:"true"  description:"Pipeline configuration file, \"-\" stands for stdin"`

	Var          []flaghelpers.VariablePairFlag     `short:"v"  long:"var"           unquote:"false"  value-name:"[NAME=STRING]"  description:"Specify a string value to set for a variable in the pipeline"`
	YAMLVar      []flaghelpers.YAMLVariablePairFlag `short:"y"  long:"yaml-var"      unquote:"false"  value-name:"[NAME=YAML]"    description:"Specify a YAML value to set for a variable in the pipeline"`
	InstanceVars []flaghelpers.YAMLVariablePairFlag `short:"i"  long:"instance-var"  unquote:"false"  hidden:"true"  value-name:"[NAME=STRING]"  description:"Specify a YAML value to set for an instance variable"`

	VarsFrom 	 []atc.PathFlag `short:"l"  long:"load-vars-from"  description:"Variable flag that can be used for filling in template values in configuration from a YAML file"`

	Team 		 flaghelpers.TeamFlag `long:"team"              description:"Name of the team to which the pipeline belongs, if different from the target default"`
}

func (command *SetPipelineCommand) Validate() ([]concourse.ConfigWarning, error) {
	var warnings []concourse.ConfigWarning
	var err error
	if strings.Contains(command.PipelineName, "/") {
		err = errors.New("pipeline name cannot contain '/'")
	}
	if string(command.Team) != "" {
		var warning *atc.ConfigWarning
		if warning, err = atc.ValidateIdentifier(string(command.Team), "team"); warning != nil {
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
	team, err = command.Team.LoadTeam(target)
	if err != nil {
		return err
	}

	ansi.DisableColors(command.DisableAnsiColor)

	var instanceVars atc.InstanceVars
	if len(command.InstanceVars) != 0 {
		var kvPairs vars.KVPairs
		for _, iv := range command.InstanceVars {
			kvPairs = append(kvPairs, vars.KVPair(iv))
		}
		instanceVars = atc.InstanceVars(kvPairs.Expand())
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
		GivenTeamName:    string(command.Team),
	}

	yamlTemplateWithParams := templatehelpers.NewYamlTemplateWithParams(configPath, templateVariablesFiles, command.Var, command.YAMLVar, instanceVars)
	return atcConfig.Set(yamlTemplateWithParams)
}
