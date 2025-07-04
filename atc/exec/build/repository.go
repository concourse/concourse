package build

import (
	"maps"
	"sync"

	"github.com/concourse/concourse/atc/runtime"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// ArtifactName is just a string, with its own type to make interfaces using it
// more self-documenting.
type ArtifactName string

type ArtifactEntry struct {
	Artifact  runtime.Artifact
	FromCache bool
}

// Repository is the mapping from a ArtifactName to an Artifact.
// Steps will both populate this map with new artifacts (e.g. the resource
// fetched by a Get step), and look up required artifacts (e.g. the inputs
// configured for a Task step).
//
// There is only one ArtifactRepository for the duration of a build plan's
// execution.
type Repository struct {
	repo   sync.Map
	parent *Repository
}

// NewRepository constructs a new repository.
func NewRepository() *Repository {
	return &Repository{}
}

// RegisterArtifact inserts an Artifact into the map under the given
// ArtifactName. Producers of artifacts, e.g. the Get step and the Task step,
// will call this after they've successfully produced their artifact(s).
func (repo *Repository) RegisterArtifact(name ArtifactName, artifact runtime.Artifact, fromCache bool) {
	repo.repo.Store(name, ArtifactEntry{
		Artifact:  artifact,
		FromCache: fromCache,
	})
}

// ArtifactFor looks up the Artifact for a given ArtifactName. Consumers of
// artifacts, e.g. the Task step, will call this to locate their dependencies.
func (repo *Repository) ArtifactFor(name ArtifactName) (runtime.Artifact, bool, bool) {
	value, found := repo.repo.Load(name)
	if !found && repo.parent != nil {
		return repo.parent.ArtifactFor(name)
	}
	if !found {
		return nil, false, false
	}
	artifactEntry := value.(ArtifactEntry)
	return artifactEntry.Artifact, artifactEntry.FromCache, true
}

// AsMap extracts the current contents of the ArtifactRepository into a new map
// and returns it. Changes to the returned map or the ArtifactRepository will not
// affect each other.
func (repo *Repository) AsMap() map[ArtifactName]ArtifactEntry {
	result := make(map[ArtifactName]ArtifactEntry)

	if repo.parent != nil {
		maps.Copy(result, repo.parent.AsMap())
	}

	repo.repo.Range(func(key, value any) bool {
		name := key.(ArtifactName)
		artifactEntry := value.(ArtifactEntry)
		result[name] = artifactEntry
		return true
	})

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
