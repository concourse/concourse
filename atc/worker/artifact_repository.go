package worker

import (
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
