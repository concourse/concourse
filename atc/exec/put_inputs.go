package exec

import (
	"fmt"

	"github.com/concourse/concourse/atc/runtime"

	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/resource"
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

type putInput struct {
	name     build.ArtifactName
	artifact runtime.Artifact
}

func (input putInput) Artifact() runtime.Artifact { return input.artifact }

func (input putInput) DestinationPath() string {
	return resource.ResourcesDir("put/" + string(input.name))
}
