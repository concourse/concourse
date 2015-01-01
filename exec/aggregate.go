package exec

import (
	"io"
	"os"

	"github.com/tedsuo/ifrit/grouper"
)

type Aggregate map[string]Step

func (a Aggregate) Using(source ArtifactSource) ArtifactSource {
	sources := aggregateArtifactSource{}

	for name, step := range a {
		sources[name] = step.Using(source)
	}

	return sources
}

type aggregateArtifactSource map[string]ArtifactSource

func (source aggregateArtifactSource) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	members := make(grouper.Members, 0, len(source))

	for name, runner := range source {
		members = append(members, grouper.Member{
			Name:   name,
			Runner: runner,
		})
	}

	return grouper.NewParallel(os.Interrupt, members).Run(signals, ready)
}

func (source aggregateArtifactSource) StreamTo(ArtifactDestination) error {
	return nil
}

func (source aggregateArtifactSource) StreamFile(path string) (io.ReadCloser, error) {
	return nil, nil
}

func (source aggregateArtifactSource) Release() error {
	// release all sources
	return nil
}
