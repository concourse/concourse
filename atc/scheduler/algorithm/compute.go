package algorithm

import (
	"fmt"

	"github.com/concourse/concourse/atc/db"
)

type Resolver interface {
	Resolve(int) (map[string]*versionCandidate, db.ResolutionFailure, error)
	InputConfigs() InputConfigs
}

func New(versionsDB db.VersionsDB) *algorithm {
	return &algorithm{
		versionsDB: versionsDB,
	}
}

type algorithm struct {
	versionsDB db.VersionsDB
}

func (a *algorithm) Compute(
	job db.Job,
	resources db.Resources,
	relatedJobs NameToIDMap,
) (db.InputMapping, bool, error) {
	resolvers, err := constructResolvers(a.versionsDB, job, resources, relatedJobs)
	if err != nil {
		return nil, false, fmt.Errorf("construct resolvers: %w", err)
	}

	inputMapper, err := newInputMapper(a.versionsDB, job.ID())
	if err != nil {
		return nil, false, fmt.Errorf("setting up input mapper: %w", err)
	}

	finalResolved := true
	finalMapping := db.InputMapping{}
	for _, resolver := range resolvers {
		versionCandidates, resolveErr, err := resolver.Resolve(0)
		if err != nil {
			return nil, false, fmt.Errorf("resolve: %w", err)
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
