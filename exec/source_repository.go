package exec

import "sync"

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
