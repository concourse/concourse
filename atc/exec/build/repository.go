package build

import (
	"code.cloudfoundry.org/lager"
	"context"
	"github.com/concourse/baggageclaim"
	"strings"
	"sync"
	"io"

	"github.com/concourse/concourse/atc/runtime"
)

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
	repo  map[ArtifactName]runtime.Artifact
	repoL sync.RWMutex
}

// NewArtifactRepository constructs a new repository.
func NewRepository() *Repository {
	return &Repository{
		repo: make(map[ArtifactName]runtime.Artifact),
	}
}

//go:generate counterfeiter . RegisterableArtifact
// A RegisterableArtifact is an Artifact which can be added to the registry
type RegisterableArtifact interface {
	runtime.Artifact
}

// RegisterSource inserts an ArtifactSource into the map under the given
// ArtifactName. Producers of artifacts, e.g. the Get step and the Task step,
// will call this after they've successfully produced their artifact(s).
func (repo *Repository) RegisterArtifact(name ArtifactName, artifact RegisterableArtifact) {
	repo.repoL.Lock()
	repo.repo[name] = artifact
	repo.repoL.Unlock()
}

// SourceFor looks up a Source for the given ArtifactName. Consumers of
// artifacts, e.g. the Task step, will call this to locate their dependencies.
func (repo *Repository) ArtifactFor(name ArtifactName) (runtime.Artifact, bool) {
	repo.repoL.RLock()
	artifact, found := repo.repo[name]
	repo.repoL.RUnlock()
	return artifact, found
}

// AsMap extracts the current contents of the ArtifactRepository into a new map
// and returns it. Changes to the returned map or the ArtifactRepository will not
// affect each other.
func (repo *Repository) AsMap() map[ArtifactName]runtime.Artifact {
	result := make(map[ArtifactName]runtime.Artifact)

	repo.repoL.RLock()
	for name, artifact := range repo.repo {
		result[name] = artifact
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
