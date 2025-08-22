package commands

import (
	"errors"
	"fmt"
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
	DryRun           bool `short:"d"  long:"dry-run"               description:"Run a set pipeline step but in dry-run mode"`

	CheckCredentials bool `long:"check-creds"  description:"Validate credential variables against credential manager"`

	PipelineName string       `short:"p"  long:"pipeline"  required:"true"  description:"Pipeline to configure"`
	Config       atc.PathFlag `short:"c"  long:"config"    required:"true"  description:"Pipeline configuration file, \"-\" stands for stdin"`

	Var          []flaghelpers.VariablePairFlag     `short:"v"  long:"var"           unquote:"false"  value-name:"[NAME=STRING]"  description:"Specify a string value to set for a variable in the pipeline"`
	YAMLVar      []flaghelpers.YAMLVariablePairFlag `short:"y"  long:"yaml-var"      unquote:"false"  value-name:"[NAME=YAML]"    description:"Specify a YAML value to set for a variable in the pipeline"`
	InstanceVars []flaghelpers.YAMLVariablePairFlag `short:"i"  long:"instance-var"  unquote:"false"  value-name:"[NAME=STRING]"  description:"Specify a YAML value to set for an instance variable"`

	VarsFrom []atc.PathFlag `short:"l"  long:"load-vars-from"  description:"Variable flag that can be used for filling in template values in configuration from a YAML file"`

	Team flaghelpers.TeamFlag `long:"team"              description:"Name of the team to which the pipeline belongs, if different from the target default"`
}

func (command *SetPipelineCommand) Validate() ([]atc.ConfigErrors, error) {
	var configErrors []atc.ConfigErrors
	var err error
	if strings.Contains(command.PipelineName, "/") {
		err = errors.New("pipeline name cannot contain '/'")
	}

	if string(command.Team) != "" {
		var configError *atc.ConfigErrors
		if configError = atc.ValidateIdentifier(string(command.Team), "team"); configError != nil {
			configErrors = append(configErrors, atc.ConfigErrors{
				Type:    configError.Type,
				Message: configError.Message,
			})
		}
	}
	return configErrors, err
}

func (command *SetPipelineCommand) Execute(args []string) error {
	configErrors, err := command.Validate()

	var errorMessages []string
	for _, configError := range configErrors {
		errorMessages = append(errorMessages, configError.Message)
	}

	if errorMessages != nil {
		return fmt.Errorf("configuration invalid: %s", strings.Join(errorMessages, ", "))
	}

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
		DryRun:           command.DryRun,
		CommandErrors:    configErrors,
		GivenTeamName:    string(command.Team),
	}

	yamlTemplateWithParams := templatehelpers.NewYamlTemplateWithParams(configPath, templateVariablesFiles, command.Var, command.YAMLVar, instanceVars)
	return atcConfig.Set(yamlTemplateWithParams)
}
