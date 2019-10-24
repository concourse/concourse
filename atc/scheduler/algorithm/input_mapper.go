package algorithm

import (
	"github.com/concourse/concourse/atc/db"
)

type inputMapper struct {
	latestBuildOutputs map[string]db.ResourceVersion

	hasLatestBuild bool
}

func newInputMapper(vdb db.VersionsDB, currentJobID int) (inputMapper, error) {
	latestBuildID, found, err := vdb.LatestBuildID(currentJobID)
	if err != nil {
		return inputMapper{}, err
	}

	outputs, err := vdb.BuildOutputs(latestBuildID)
	if err != nil {
		return inputMapper{}, err
	}

	latestBuildOutputs := map[string]db.ResourceVersion{}
	for _, o := range outputs {
		latestBuildOutputs[o.InputName] = o.Version
	}

	return inputMapper{
		hasLatestBuild:     found,
		latestBuildOutputs: latestBuildOutputs,
	}, nil
}

func (m *inputMapper) candidatesToInputMapping(mapping db.InputMapping, inputConfigs InputConfigs, candidates map[string]*versionCandidate, resolveErr db.ResolutionFailure) db.InputMapping {
	for _, input := range inputConfigs {
		if resolveErr != "" {
			mapping[input.Name] = db.InputResult{
				ResolveError: resolveErr,
			}
		} else {
			mapping[input.Name] = db.InputResult{
				Input: &db.AlgorithmInput{
					AlgorithmVersion: db.AlgorithmVersion{
						ResourceID: input.ResourceID,
						Version:    candidates[input.Name].Version,
					},
					FirstOccurrence: !m.hasLatestBuild || m.latestBuildOutputs[input.Name] != candidates[input.Name].Version,
				},
				PassedBuildIDs: candidates[input.Name].SourceBuildIds,
			}
		}
	}

	return mapping
}
