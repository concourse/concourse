package algorithm

import (
	"github.com/concourse/concourse/atc/db"
)

type Resolver interface {
	Resolve(int) (map[string]*versionCandidate, db.ResolutionFailure, error)
	InputConfigs() InputConfigs
}

func New() *algorithm {
	return &algorithm{}
}

type algorithm struct{}

func (a *algorithm) Compute(
	versions *db.VersionsDB,
	job db.Job,
	resources db.Resources,
) (db.InputMapping, bool, error) {
	resolvers, err := constructResolvers(versions, job, resources)
	if err != nil {
		return nil, false, err
	}

	inputMapper, err := newInputMapper(versions, job.ID())
	if err != nil {
		return nil, false, err
	}

	finalResolved := true
	finalMapping := db.InputMapping{}
	for _, resolver := range resolvers {
		versionCandidates, resolveErr, err := resolver.Resolve(0)
		if err != nil {
			return nil, false, err
		}

		var resolved bool
		if resolveErr == "" {
			resolved = true
		}

		finalResolved = finalResolved && resolved
		finalMapping = inputMapper.candidatesToInputMapping(finalMapping, resolver.InputConfigs(), versionCandidates, resolveErr)
	}

	return finalMapping, finalResolved, nil
}
