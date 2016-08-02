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
	Pipe atc.Pipe

	BuildInput atc.BuildInput
}

func DetermineInputs(
	client concourse.Client,
	team concourse.Team,
	taskInputs []atc.TaskInputConfig,
	inputMappings []flaghelpers.InputPairFlag,
	inputsFrom flaghelpers.JobFlag,
) ([]Input, error) {
	err := CheckForUnknownInputMappings(inputMappings, taskInputs)
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

	inputsFromLocal, err := GenerateLocalInputs(client, inputMappings)
	if err != nil {
		return nil, err
	}

	inputsFromJob, err := FetchInputsFromJob(team, inputsFrom)
	if err != nil {
		return nil, err
	}

	inputs := []Input{}
	for _, taskInput := range taskInputs {
		input, found := inputsFromLocal[taskInput.Name]
		if !found {
			input, found = inputsFromJob[taskInput.Name]
			if !found {
				return nil, fmt.Errorf("missing required input `%s`", taskInput.Name)
			}
		}

		inputs = append(inputs, input)
	}

	return inputs, nil
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

func GenerateLocalInputs(client concourse.Client, inputMappings []flaghelpers.InputPairFlag) (map[string]Input, error) {
	kvMap := map[string]Input{}

	for _, i := range inputMappings {
		inputName := i.Name
		absPath := i.Path

		pipe, err := client.CreatePipe()
		if err != nil {
			return nil, err
		}

		kvMap[inputName] = Input{
			Name: inputName,
			Path: absPath,
			Pipe: pipe,
		}
	}

	return kvMap, nil
}

func FetchInputsFromJob(team concourse.Team, inputsFrom flaghelpers.JobFlag) (map[string]Input, error) {
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

	for _, buildInput := range buildInputs {
		kvMap[buildInput.Name] = Input{
			Name:       buildInput.Name,
			BuildInput: buildInput,
		}
	}

	return kvMap, nil
}
