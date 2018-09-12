package executehelpers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/go-concourse/concourse"
)

type Input struct {
	Name string
	Path string

	Plan atc.Plan
}

func DetermineInputs(
	fact atc.PlanFactory,
	team concourse.Team,
	taskInputs []atc.TaskInputConfig,
	inputMappings []flaghelpers.InputPairFlag,
	inputsFrom flaghelpers.JobFlag,
) ([]Input, error) {
	err := CheckForUnknownInputMappings(inputMappings, taskInputs)
	if err != nil {
		return nil, err
	}

	err = CheckForInputType(inputMappings)
	if err != nil {
		return nil, err
	}

	if len(inputMappings) == 0 && inputsFrom.PipelineName == "" && inputsFrom.JobName == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}

		inputMappings = append(inputMappings, flaghelpers.InputPairFlag{
			Name: filepath.Base(wd),
			Path: wd,
		})
	}

	inputsFromLocal, err := GenerateLocalInputs(fact, inputMappings)
	if err != nil {
		return nil, err
	}

	inputsFromJob, err := FetchInputsFromJob(fact, team, inputsFrom)
	if err != nil {
		return nil, err
	}

	inputs := []Input{}
	for _, taskInput := range taskInputs {
		input, found := inputsFromLocal[taskInput.Name]
		if !found {
			input, found = inputsFromJob[taskInput.Name]
			if !found {
				if taskInput.Optional {
					continue
				} else {
					return nil, fmt.Errorf("missing required input `%s`", taskInput.Name)
				}
			}
		}

		inputs = append(inputs, input)
	}

	return inputs, nil
}

func CheckForInputType(inputMaps []flaghelpers.InputPairFlag) error {
	for _, i := range inputMaps {
		if i.Path != "" {
			fi, err := os.Stat(i.Path)
			if err != nil {
				return err
			}
			switch mode := fi.Mode(); {
			case mode.IsRegular():
				return errors.New(i.Path + " not a folder")
			}
		}
	}
	return nil
}

func CheckForUnknownInputMappings(inputMappings []flaghelpers.InputPairFlag, validInputs []atc.TaskInputConfig) error {
	for _, inputMapping := range inputMappings {
		if !TaskInputsContainsName(validInputs, inputMapping.Name) {
			return fmt.Errorf("unknown input `%s`", inputMapping.Name)
		}
	}
	return nil
}

func TaskInputsContainsName(inputs []atc.TaskInputConfig, name string) bool {
	for _, input := range inputs {
		if input.Name == name {
			return true
		}
	}
	return false
}

func GenerateLocalInputs(fact atc.PlanFactory, inputMappings []flaghelpers.InputPairFlag) (map[string]Input, error) {
	kvMap := map[string]Input{}

	for _, i := range inputMappings {
		inputName := i.Name
		absPath := i.Path

		kvMap[inputName] = Input{
			Name: inputName,
			Path: absPath,
			Plan: fact.NewPlan(atc.UserArtifactPlan{
				Name: inputName,
			}),
		}
	}

	return kvMap, nil
}

func FetchInputsFromJob(fact atc.PlanFactory, team concourse.Team, inputsFrom flaghelpers.JobFlag) (map[string]Input, error) {
	kvMap := map[string]Input{}
	if inputsFrom.PipelineName == "" && inputsFrom.JobName == "" {
		return kvMap, nil
	}

	buildInputs, found, err := team.BuildInputsForJob(inputsFrom.PipelineName, inputsFrom.JobName)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, errors.New("build inputs not found")
	}

	versionedResourceTypes, found, err := team.VersionedResourceTypes(inputsFrom.PipelineName)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, errors.New("versioned resource types not found")
	}

	for _, buildInput := range buildInputs {
		version := buildInput.Version

		kvMap[buildInput.Name] = Input{
			Name: buildInput.Name,

			Plan: fact.NewPlan(atc.GetPlan{
				Name:                   buildInput.Name,
				Type:                   buildInput.Type,
				Source:                 buildInput.Source,
				Version:                &version,
				Params:                 buildInput.Params,
				Tags:                   buildInput.Tags,
				VersionedResourceTypes: versionedResourceTypes,
			}),
		}
	}

	return kvMap, nil
}
