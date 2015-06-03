package exec

import (
	"io"
	"strings"
	"sync"
)

type SourceRepository struct {
	repo  map[SourceName]ArtifactSource
	repoL sync.RWMutex
}

func NewSourceRepository() *SourceRepository {
	return &SourceRepository{
		repo: make(map[SourceName]ArtifactSource),
	}
}

func (repo *SourceRepository) RegisterSource(name SourceName, source ArtifactSource) {
	repo.repoL.Lock()
	repo.repo[name] = source
	repo.repoL.Unlock()
}

func (repo *SourceRepository) SourceFor(name SourceName) (ArtifactSource, bool) {
	repo.repoL.RLock()
	source, found := repo.repo[name]
	repo.repoL.RUnlock()
	return source, found
}

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

type subdirectoryDestination struct {
	destination  ArtifactDestination
	subdirectory string
}

func (dest subdirectoryDestination) StreamIn(dst string, src io.Reader) error {
	return dest.destination.StreamIn(dest.subdirectory+"/"+dst, src)
}
