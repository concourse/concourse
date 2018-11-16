package exec

import (
	"fmt"

	"code.cloudfoundry.org/lager"
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
	FindAll(*worker.ArtifactRepository) ([]worker.InputSource, error)
}

type allInputs struct{}

func NewAllInputs() PutInputs {
	return &allInputs{}
}

func (i allInputs) FindAll(artifacts *worker.ArtifactRepository) ([]worker.InputSource, error) {
	inputs := []worker.InputSource{}

	for name, source := range artifacts.AsMap() {
		inputs = append(inputs, &putInputSource{
			name:   name,
			source: PutResourceSource{source},
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

func (i specificInputs) FindAll(artifacts *worker.ArtifactRepository) ([]worker.InputSource, error) {
	artifactsMap := artifacts.AsMap()

	inputs := []worker.InputSource{}
	for _, i := range i.inputs {
		artifactSource, found := artifactsMap[worker.ArtifactName(i)]
		if !found {
			return nil, PutInputNotFoundError{Input: i}
		}

		inputs = append(inputs, &putInputSource{
			name:   worker.ArtifactName(i),
			source: PutResourceSource{artifactSource},
		})
	}

	return inputs, nil
}

type putInputSource struct {
	name   worker.ArtifactName
	source worker.ArtifactSource
}

func (s *putInputSource) Source() worker.ArtifactSource { return s.source }

func (s *putInputSource) DestinationPath() string {
	return resource.ResourcesDir("put/" + string(s.name))
}

type PutResourceSource struct {
	worker.ArtifactSource
}

func (source PutResourceSource) StreamTo(logger lager.Logger, dest worker.ArtifactDestination) error {
	return source.ArtifactSource.StreamTo(logger, worker.ArtifactDestination(dest))
}
