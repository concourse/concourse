package exec

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/concourse/atc/worker"
)

// SourceName is just a string, with its own type to make interfaces using it
// more self-documenting.
type SourceName string

// SourceRepository is the mapping from a SourceName to an ArtifactSource.
// Steps will both populate this map with new artifacts (e.g.  the resource
// fetched by a Get step), and look up required artifacts (e.g.  the inputs
// configured for a Task step).
//
// There is only one SourceRepository for the duration of a build plan's
// execution.
//
// SourceRepository is, itself, an ArtifactSource. As an ArtifactSource it acts
// as the set of all ArtifactSources it contains, as if they were each in
// subdirectories corresponding to their SourceName.
type SourceRepository struct {
	repo  map[SourceName]ArtifactSource
	repoL sync.RWMutex
}

// NewSourceRepository constructs a new repository.
func NewSourceRepository() *SourceRepository {
	return &SourceRepository{
		repo: make(map[SourceName]ArtifactSource),
	}
}

// RegisterSource inserts an ArtifactSource into the map under the given
// SourceName. Producers of artifacts, e.g. the Get step and the Task step,
// will call this after they've successfully produced their artifact(s).
func (repo *SourceRepository) RegisterSource(name SourceName, source ArtifactSource) {
	repo.repoL.Lock()
	repo.repo[name] = source
	repo.repoL.Unlock()
}

// SourceFor looks up an ArtifactSource for the given SourceName. Consumers of
// artifacts, e.g. the Task step, will call this to locate their dependencies.
func (repo *SourceRepository) SourceFor(name SourceName) (ArtifactSource, bool) {
	repo.repoL.RLock()
	source, found := repo.repo[name]
	repo.repoL.RUnlock()
	return source, found
}

// StreamTo will stream all currently registered artifacts to the destination.
// This is used by the Put step, which currently does not have an explicit set
// of dependencies, and instead just pulls in everything.
//
// Each ArtifactSource will be streamed to a subdirectory matching its
// SourceName.
func (repo *SourceRepository) StreamTo(dest ArtifactDestination) error {
	sources := map[SourceName]ArtifactSource{}

	repo.repoL.RLock()
	for k, v := range repo.repo {
		sources[k] = v
	}
	repo.repoL.RUnlock()

	for name, src := range sources {
		err := src.StreamTo(subdirectoryDestination{dest, string(name)})
		if err != nil {
			return err
		}
	}

	return nil
}

// StreamFile streams a single file out of the repository, using the first path
// segment to determine the ArtifactSource to stream out of. For example,
// StreamFile("a/b.yml") will look up the "a" ArtifactSource and return the
// result of StreamFile("b.yml") on it.
//
// If the ArtifactSource determined by the path is not present,
// FileNotFoundError will be returned.
func (repo *SourceRepository) StreamFile(path string) (io.ReadCloser, error) {
	sources := map[SourceName]ArtifactSource{}

	repo.repoL.RLock()
	for k, v := range repo.repo {
		sources[k] = v
	}
	repo.repoL.RUnlock()

	for name, src := range sources {
		if strings.HasPrefix(path, string(name)+"/") {
			return src.StreamFile(path[len(name)+1:])
		}
	}

	return nil, FileNotFoundError{Path: path}
}

// VolumeOn returns nothing, as it's impossible for there to be a single volume
// representing all ArtifactSources.
func (repo *SourceRepository) VolumeOn(worker worker.Worker) (worker.Volume, bool, error) {
	return nil, false, nil
}

// ScopedTo returns a new SourceRepository restricted to the given set of
// SourceNames. This is used by the Put step to stream in the sources that did
// not have a volume available on its destination.
func (repo *SourceRepository) ScopedTo(names ...SourceName) (*SourceRepository, error) {
	newRepo := NewSourceRepository()

	for _, name := range names {
		source, found := repo.SourceFor(name)
		if !found {
			return nil, fmt.Errorf("source does not exist in repository: %s", name)
		}

		newRepo.RegisterSource(name, source)
	}

	return newRepo, nil
}

// AsMap extracts the current contents of the SourceRepository into a new map
// and returns it. Changes to the returned map or the SourceRepository will not
// affect each other.
func (repo *SourceRepository) AsMap() map[SourceName]ArtifactSource {
	result := make(map[SourceName]ArtifactSource)

	repo.repoL.RLock()
	for name, source := range repo.repo {
		result[name] = source
	}
	repo.repoL.RUnlock()

	return result
}

type subdirectoryDestination struct {
	destination  ArtifactDestination
	subdirectory string
}

func (dest subdirectoryDestination) StreamIn(dst string, src io.Reader) error {
	return dest.destination.StreamIn(dest.subdirectory+"/"+dst, src)
}
