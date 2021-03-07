package build

import (
	"sync"

	"github.com/concourse/concourse/atc/runtime"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// ArtifactName is just a string, with its own type to make interfaces using it
// more self-documenting.
type ArtifactName string

// Repository is the mapping from a ArtifactName to an Artifact.
// Steps will both populate this map with new artifacts (e.g.  the resource
// fetched by a Get step), and look up required artifacts (e.g.  the inputs
// configured for a Task step).
//
// There is only one ArtifactRepository for the duration of a build plan's
// execution.
//
type Repository struct {
	repo  map[ArtifactName]runtime.Volume
	repoL sync.RWMutex

	parent *Repository
}

// NewRepository constructs a new repository.
func NewRepository() *Repository {
	return &Repository{
		repo: make(map[ArtifactName]runtime.Volume),
	}
}

// RegisterArtifact inserts an artifact Volume into the map under the given
// ArtifactName. Producers of artifacts, e.g. the Get step and the Task step,
// will call this after they've successfully produced their artifact(s).
func (repo *Repository) RegisterArtifact(name ArtifactName, volume runtime.Volume) {
	repo.repoL.Lock()
	repo.repo[name] = volume
	repo.repoL.Unlock()
}

// SourceFor looks up a Source for the given ArtifactName. Consumers of
// artifacts, e.g. the Task step, will call this to locate their dependencies.
func (repo *Repository) ArtifactFor(name ArtifactName) (runtime.Volume, bool) {
	repo.repoL.RLock()
	artifact, found := repo.repo[name]
	repo.repoL.RUnlock()
	if !found && repo.parent != nil {
		artifact, found = repo.parent.ArtifactFor(name)
	}
	return artifact, found
}

// AsMap extracts the current contents of the ArtifactRepository into a new map
// and returns it. Changes to the returned map or the ArtifactRepository will not
// affect each other.
func (repo *Repository) AsMap() map[ArtifactName]runtime.Volume {
	result := make(map[ArtifactName]runtime.Volume)

	if repo.parent != nil {
		for name, artifact := range repo.parent.AsMap() {
			result[name] = artifact
		}
	}

	repo.repoL.RLock()
	for name, artifact := range repo.repo {
		result[name] = artifact
	}
	repo.repoL.RUnlock()

	return result
}

func (repo *Repository) NewLocalScope() *Repository {
	child := NewRepository()
	child.parent = repo
	return child
}

func (repo *Repository) Parent() *Repository {
	return repo.parent
}
