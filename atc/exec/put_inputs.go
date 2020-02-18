package exec

import (
	"fmt"
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
	FindAll(*build.Repository) (map[string]runtime.Artifact, error)
}

type allInputs struct{}

func NewAllInputs() PutInputs {
	return &allInputs{}
}

func (i allInputs) FindAll(artifacts *build.Repository) (map[string]runtime.Artifact, error) {
	inputs := map[string]runtime.Artifact{}

	for name, artifact := range artifacts.AsMap() {
		pi := putInput{
			name:     name,
			artifact: artifact,
		}

		inputs[pi.DestinationPath()] = pi.Artifact()
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

func (i specificInputs) FindAll(artifacts *build.Repository) (map[string]runtime.Artifact, error) {
	artifactsMap := artifacts.AsMap()

	inputs := map[string]runtime.Artifact{}

	for _, i := range i.inputs {
		artifact, found := artifactsMap[build.ArtifactName(i)]
		if !found {
			return nil, PutInputNotFoundError{Input: i}
		}

		pi := putInput{
			name:     build.ArtifactName(i),
			artifact: artifact,
		}

		inputs[pi.DestinationPath()] = pi.Artifact()
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
		if idx := strings.IndexByte(actual, '/'); idx >= 0 {
			input = actual[:idx]
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

func (i detectInputs) FindAll(artifacts *build.Repository) (map[string]runtime.Artifact, error) {
	artifactsMap := artifacts.AsMap()

	inputs := map[string]runtime.Artifact{}
	for _, name := range i.guessedNames {
		artifact, found := artifactsMap[name]
		if !found {
			// false positive; not an artifact
			continue
		}

		pi := putInput{
			name:     name,
			artifact: artifact,
		}

		inputs[pi.DestinationPath()] = pi.Artifact()
	}

	return inputs, nil
}

type putInput struct {
	name     build.ArtifactName
	artifact runtime.Artifact
}

func (input putInput) Artifact() runtime.Artifact { return input.artifact }

func (input putInput) DestinationPath() string {
	return resource.ResourcesDir("put/" + string(input.name))
}
