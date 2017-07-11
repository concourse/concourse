package worker

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

// ArtifactName is just a string, with its own type to make interfaces using it
// more self-documenting.
type ArtifactName string

// ArtifactRepository is the mapping from a ArtifactName to an ArtifactSource.
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
type ArtifactRepository struct {
	repo  map[ArtifactName]ArtifactSource
	repoL sync.RWMutex
}

// NewArtifactRepository constructs a new repository.
func NewArtifactRepository() *ArtifactRepository {
	return &ArtifactRepository{
		repo: make(map[ArtifactName]ArtifactSource),
	}
}

// RegisterSource inserts an ArtifactSource into the map under the given
// ArtifactName. Producers of artifacts, e.g. the Get step and the Task step,
// will call this after they've successfully produced their artifact(s).
func (repo *ArtifactRepository) RegisterSource(name ArtifactName, source ArtifactSource) {
	repo.repoL.Lock()
	repo.repo[name] = source
	repo.repoL.Unlock()
}

// SourceFor looks up a Source for the given ArtifactName. Consumers of
// artifacts, e.g. the Task step, will call this to locate their dependencies.
func (repo *ArtifactRepository) SourceFor(name ArtifactName) (ArtifactSource, bool) {
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
// ArtifactName.
func (repo *ArtifactRepository) StreamTo(dest ArtifactDestination) error {
	sources := map[ArtifactName]ArtifactSource{}

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
func (repo *ArtifactRepository) StreamFile(path string) (io.ReadCloser, error) {
	sources := map[ArtifactName]ArtifactSource{}

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
func (repo *ArtifactRepository) VolumeOn(worker Worker) (Volume, bool, error) {
	return nil, false, nil
}

// ScopedTo returns a new ArtifactRepository restricted to the given set of
// ArtifactNames. This is used by the Put step to stream in the sources that did
// not have a volume available on its destination.
func (repo *ArtifactRepository) ScopedTo(names ...ArtifactName) (*ArtifactRepository, error) {
	newRepo := NewArtifactRepository()

	for _, name := range names {
		source, found := repo.SourceFor(name)
		if !found {
			return nil, fmt.Errorf("source does not exist in repository: %s", name)
		}

		newRepo.RegisterSource(name, source)
	}

	return newRepo, nil
}

// AsMap extracts the current contents of the ArtifactRepository into a new map
// and returns it. Changes to the returned map or the ArtifactRepository will not
// affect each other.
func (repo *ArtifactRepository) AsMap() map[ArtifactName]ArtifactSource {
	result := make(map[ArtifactName]ArtifactSource)

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

// FileNotFoundError is the error to return from StreamFile when the given path
// does not exist.
type FileNotFoundError struct {
	Path string
}

// Error prints a helpful message including the file path. The user will see
// this message if e.g. their task config path does not exist.
func (err FileNotFoundError) Error() string {
	return fmt.Sprintf("file not found: %s", err.Path)
}
