package artifact

import (
	"code.cloudfoundry.org/lager"
	"context"
	"github.com/concourse/baggageclaim"
	"strings"
	"sync"
	"io"

	"github.com/concourse/concourse/atc/worker"
)

// Name is just a string, with its own type to make interfaces using it
// more self-documenting.
type Name string

// Repository is the mapping from a Name to an ArtifactSource.
// Steps will both populate this map with new artifacts (e.g.  the resource
// fetched by a Get step), and look up required artifacts (e.g.  the inputs
// configured for a Task step).
//
// There is only one ArtifactRepository for the duration of a build plan's
// execution.
//
// ArtifactRepository is, itself, an ArtifactSource. As an ArtifactSource it acts
// as the set of all ArtifactSources it contains, as if they were each in
// subdirectories corresponding to their ArtifactName.
type Repository struct {
	repo  map[Name]worker.ArtifactSource
	repoL sync.RWMutex
}

// NewArtifactRepository constructs a new repository.
func NewRepository() *Repository {
	return &Repository{
		repo: make(map[Name]worker.ArtifactSource),
	}
}

//go:generate counterfeiter . RegisterableSource
// A RegisterableSource	artifact is an ArtifactSource which can be added to the registry
type RegisterableSource interface {
	worker.ArtifactSource
}

// RegisterSource inserts an ArtifactSource into the map under the given
// ArtifactName. Producers of artifacts, e.g. the Get step and the Task step,
// will call this after they've successfully produced their artifact(s).
func (repo *Repository) RegisterSource(name Name, source RegisterableSource) {
	repo.repoL.Lock()
	repo.repo[name] = source
	repo.repoL.Unlock()
}

// SourceFor looks up a Source for the given ArtifactName. Consumers of
// artifacts, e.g. the Task step, will call this to locate their dependencies.
func (repo *Repository) SourceFor(name Name) (worker.ArtifactSource, bool) {
	repo.repoL.RLock()
	source, found := repo.repo[name]
	repo.repoL.RUnlock()
	return source, found
}

// AsMap extracts the current contents of the ArtifactRepository into a new map
// and returns it. Changes to the returned map or the ArtifactRepository will not
// affect each other.
func (repo *Repository) AsMap() map[Name]worker.ArtifactSource {
	result := make(map[Name]worker.ArtifactSource)

	repo.repoL.RLock()
	for name, source := range repo.repo {
		result[name] = source
	}
	repo.repoL.RUnlock()

	return result
}

func (repo *Repository) StreamFile(ctx context.Context, logger lager.Logger, path string) (io.ReadCloser, error) {
	segs := strings.SplitN(path, "/", 2)
	if len(segs) != 2 {
		return nil, UnspecifiedArtifactSourceError{
			Path: path,
		}
	}

	sourceName := Name(segs[0])
	filePath := segs[1]

	source, found := repo.SourceFor(sourceName)
	if !found {
		return nil, UnknownArtifactSourceError{
			Name: sourceName,
			Path: path,
		}
	}

	stream, err := source.StreamFile(ctx, logger, filePath)
	if err != nil {
		if err == baggageclaim.ErrFileNotFound {
			return nil, FileNotFoundError{
				Name:     sourceName,
				FilePath: filePath,
			}
		}

		return nil, err
	}

	return stream, nil
}
