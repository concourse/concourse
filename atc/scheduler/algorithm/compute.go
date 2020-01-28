package algorithm

import (
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

type Resolver interface {
	Resolve(int) (map[string]*versionCandidate, db.ResolutionFailure, error)
	InputConfigs() InputConfigs
}

func New(versionsDB db.VersionsDB) *Algorithm {
	return &Algorithm{
		versionsDB: versionsDB,
	}
}

type Algorithm struct {
	versionsDB db.VersionsDB
}

func (a *Algorithm) Compute(
	job db.Job,
	inputs []atc.JobInput,
	resources db.Resources,
	relatedJobs NameToIDMap,
) (db.InputMapping, bool, bool, error) {
	resolvers, err := constructResolvers(a.versionsDB, job, inputs, resources, relatedJobs)
	if err != nil {
		return nil, false, false, fmt.Errorf("construct resolvers: %w", err)
	}

	inputMapper, err := newInputMapper(a.versionsDB, job.ID())
	if err != nil {
		return nil, false, false, fmt.Errorf("setting up input mapper: %w", err)
	}

	return a.computeResolvers(resolvers, inputMapper)
}

func (a *Algorithm) computeResolvers(resolvers []Resolver, inputMapper inputMapper) (db.InputMapping, bool, bool, error) {
	finalHasNext := false
	finalResolved := true
	finalMapping := db.InputMapping{}

	for _, resolver := range resolvers {
		versionCandidates, resolveErr, err := resolver.Resolve(0)
		if err != nil {
			return nil, false, false, fmt.Errorf("resolve: %w", err)
		}

		// determines if the algorithm successfully resolved all inputs depending
		// on if all resolvers did not return a resolve error
		finalResolved = finalResolved && (resolveErr == "")

		// converts the version candidates into an object that is recognizable by
		// other components. also computes the first occurrence for all satisfiable
		// inputs
		finalMapping = inputMapper.candidatesToInputMapping(finalMapping, resolver.InputConfigs(), versionCandidates, resolveErr)

		// if any one of the resolvers has a version candidate that has an unused
		// next every version, the algorithm should return true for being able to
		// be run again
		finalHasNext = finalHasNext || a.finalizeHasNext(versionCandidates)
	}

	return finalMapping, finalResolved, finalHasNext, nil
}

func (a *Algorithm) finalizeHasNext(versionCandidates map[string]*versionCandidate) bool {
	hasNextCombined := false
	for _, candidate := range versionCandidates {
		hasNextCombined = hasNextCombined || candidate.HasNextEveryVersion
	}

	return hasNextCombined
}
