package executehelpers

import (
	"fmt"
	"path/filepath"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/flaghelpers"
)

type Output struct {
	Name string
	Path string
	Plan atc.Plan
}

func DetermineOutputs(
	fact atc.PlanFactory,
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

		outputs = append(outputs, Output{
			Name: outputName,
			Path: absPath,
			Plan: fact.NewPlan(atc.ArtifactOutputPlan{
				Name: outputName,
			}),
		})
	}

	return outputs, nil
}
