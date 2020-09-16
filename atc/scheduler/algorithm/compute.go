package algorithm

import (
	"context"
	"fmt"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/tracing"
)

type Resolver interface {
	Resolve(context.Context) (map[string]*versionCandidate, db.ResolutionFailure, error)
	InputConfigs() db.InputConfigs
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
	ctx context.Context,
	job db.Job,
	inputs db.InputConfigs,
) (db.InputMapping, bool, bool, error) {
	ctx, span := tracing.StartSpan(ctx, "Algorithm.Compute", tracing.Attrs{
		"pipeline": job.PipelineName(),
		"job":      job.Name(),
	})
	defer span.End()

	resolvers, err := constructResolvers(a.versionsDB, inputs)
	if err != nil {
		return nil, false, false, fmt.Errorf("construct resolvers: %w", err)
	}

	return a.computeResolvers(ctx, resolvers)
}

func (a *Algorithm) computeResolvers(
	ctx context.Context,
	resolvers []Resolver,
) (db.InputMapping, bool, bool, error) {
	finalHasNext := false
	finalResolved := true
	finalMapping := db.InputMapping{}

	for _, resolver := range resolvers {
		versionCandidates, resolveErr, err := resolver.Resolve(ctx)
		if err != nil {
			return nil, false, false, fmt.Errorf("resolve: %w", err)
		}

		// determines if the algorithm successfully resolved all inputs depending
		// on if all resolvers did not return a resolve error
		finalResolved = finalResolved && (resolveErr == "")

		// converts the version candidates into an object that is recognizable by
		// other components. also computes the first occurrence for all satisfiable
		// inputs
		finalMapping, err = a.candidatesToInputMapping(ctx, finalMapping, resolver.InputConfigs(), versionCandidates, resolveErr)
		if err != nil {
			return nil, false, false, fmt.Errorf("candidates to input mapping: %w", err)
		}

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

func (a *Algorithm) candidatesToInputMapping(ctx context.Context, mapping db.InputMapping, inputConfigs db.InputConfigs, candidates map[string]*versionCandidate, resolveErr db.ResolutionFailure) (db.InputMapping, error) {
	for _, input := range inputConfigs {
		if resolveErr != "" {
			mapping[input.Name] = db.InputResult{
				ResolveError: resolveErr,
			}
		} else {
			firstOcc, err := a.versionsDB.IsFirstOccurrence(ctx, input.JobID, input.Name, candidates[input.Name].Version, input.ResourceID)
			if err != nil {
				return nil, err
			}

			mapping[input.Name] = db.InputResult{
				Input: &db.AlgorithmInput{
					AlgorithmVersion: db.AlgorithmVersion{
						ResourceID: input.ResourceID,
						Version:    candidates[input.Name].Version,
					},
					FirstOccurrence: firstOcc,
				},
				PassedBuildIDs: candidates[input.Name].SourceBuildIds,
			}
		}
	}

	return mapping, nil
}
