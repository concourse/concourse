package exec

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
)

type PutInputNotFoundError struct {
	Input string
}

func (e PutInputNotFoundError) Error() string {
	return fmt.Sprintf("input not found: %s", e.Input)
}

type PutInputs interface {
	FindAll(*build.Repository) ([]runtime.Input, error)
}

type allInputs struct{}

func NewAllInputs() PutInputs {
	return &allInputs{}
}

func (i allInputs) FindAll(artifacts *build.Repository) ([]runtime.Input, error) {
	artifactsMap := artifacts.AsMap()

	inputs := make([]runtime.Input, 0, len(artifactsMap))
	for name, vol := range artifactsMap {
		inputs = append(inputs, putInput(name, vol))
	}

	return inputs, nil
}

type specificInputs struct {
	inputs []string
}

func NewSpecificInputs(inputs []string) PutInputs {
	return &specificInputs{
		inputs: inputs,
	}
}

func (i specificInputs) FindAll(artifacts *build.Repository) ([]runtime.Input, error) {
	artifactsMap := artifacts.AsMap()

	inputs := make([]runtime.Input, len(i.inputs))
	for i, name := range i.inputs {
		vol, found := artifactsMap[build.ArtifactName(name)]
		if !found {
			return nil, PutInputNotFoundError{Input: name}
		}
		inputs[i] = putInput(build.ArtifactName(name), vol)
	}

	return inputs, nil
}

type detectInputs struct {
	guessedNames []build.ArtifactName
}

func detectInputsFromParam(value interface{}) []build.ArtifactName {
	switch actual := value.(type) {
	case string:
		input := actual
		if parts := strings.Split(actual, "/"); len(parts) > 1 {
			for _, part := range parts {
				if part == "." || part == ".." {
					continue
				}
				input = part
				break
			}
		}
		return []build.ArtifactName{build.ArtifactName(input)}
	case map[string]interface{}:
		var inputs []build.ArtifactName
		for _, value := range actual {
			inputs = append(inputs, detectInputsFromParam(value)...)
		}
		return inputs
	case []interface{}:
		var inputs []build.ArtifactName
		for _, value := range actual {
			inputs = append(inputs, detectInputsFromParam(value)...)
		}
		return inputs
	default:
		return []build.ArtifactName{}
	}
}

func NewDetectInputs(params atc.Params) PutInputs {
	return &detectInputs{
		guessedNames: detectInputsFromParam(map[string]interface{}(params)),
	}
}

func (i detectInputs) FindAll(artifacts *build.Repository) ([]runtime.Input, error) {
	artifactsMap := artifacts.AsMap()

	inputs := []runtime.Input{}
	for _, name := range i.guessedNames {
		vol, found := artifactsMap[name]
		if !found {
			// false positive; not an artifact
			continue
		}
		inputs = append(inputs, putInput(name, vol))
	}

	return inputs, nil
}

func putInput(name build.ArtifactName, volume runtime.Volume) runtime.Input {
	return runtime.Input{
		VolumeHandle:    volume.Handle(),
		DestinationPath: filepath.Join(resource.ResourcesDir("put"), string(name)),
	}
}
