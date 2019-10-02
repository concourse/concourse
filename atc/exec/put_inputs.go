package exec

import (
	"fmt"

	"github.com/concourse/concourse/atc/runtime"

	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
)

type PutInputNotFoundError struct {
	Input string
}

func (e PutInputNotFoundError) Error() string {
	return fmt.Sprintf("input not found: %s", e.Input)
}

type PutInputs interface {
	FindAll(*build.Repository) ([]worker.FooBarInput, error)
}

type allInputs struct{}

func NewAllInputs() PutInputs {
	return &allInputs{}
}

func (i allInputs) FindAll(artifacts *build.Repository) ([]worker.FooBarInput, error) {
	inputs := []worker.FooBarInput{}

	for name, artifact := range artifacts.AsMap() {
		inputs = append(inputs, &putFooBarInput{
			name:     name,
			artifact: artifact,
		})
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

func (i specificInputs) FindAll(artifacts *build.Repository) ([]worker.FooBarInput, error) {
	artifactsMap := artifacts.AsMap()

	inputs := []worker.FooBarInput{}
	for _, i := range i.inputs {
		artifact, found := artifactsMap[build.ArtifactName(i)]
		if !found {
			return nil, PutInputNotFoundError{Input: i}
		}

		inputs = append(inputs, &putFooBarInput{
			name:     build.ArtifactName(i),
			artifact: artifact,
		})
	}

	return inputs, nil
}

type putFooBarInput struct {
	name     build.ArtifactName
	artifact runtime.Artifact
}

func (s putFooBarInput) Artifact() runtime.Artifact { return s.artifact }

func (s putFooBarInput) DestinationPath() string {
	return resource.ResourcesDir("put/" + string(s.name))
}

//func (source PutResourceSource) StreamTo(ctx context.Context, logger lager.Logger, dest worker.ArtifactDestination) error {
//	return source.ArtifactSource.StreamTo(ctx, logger, dest)
//}
