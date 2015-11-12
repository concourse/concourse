package executehelpers

import (
	"fmt"
	"path/filepath"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/go-concourse/concourse"
)

type Output struct {
	Name string
	Path string
	Pipe atc.Pipe
}

func DetermineOutputs(
	client concourse.Client,
	taskOutputs []atc.TaskOutputConfig,
	outputMappings []flaghelpers.OutputPairFlag,
) ([]Output, error) {

	outputs := []Output{}

	for _, i := range outputMappings {
		outputName := i.Name

		notInConfig := true
		for _, configOutput := range taskOutputs {
			if configOutput.Name == outputName {
				notInConfig = false
			}
		}
		if notInConfig {
			return nil, fmt.Errorf("unknown output '%s'", outputName)
		}

		absPath, err := filepath.Abs(i.Path)
		if err != nil {
			return nil, err
		}

		pipe, err := client.CreatePipe()
		if err != nil {
			return nil, err
		}

		outputs = append(outputs, Output{
			Name: outputName,
			Path: absPath,
			Pipe: pipe,
		})
	}

	return outputs, nil
}
