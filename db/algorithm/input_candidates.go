package algorithm

import (
	"fmt"
	"strings"
)

type InputCandidates []InputVersionCandidates

type InputVersionCandidates struct {
	Input  string
	Passed JobSet

	VersionCandidates
}

func (candidates InputCandidates) String() string {
	lens := []string{}
	for _, vcs := range candidates {
		lens = append(lens, fmt.Sprintf("%s (%d candidates for %d versions)", vcs.Input, len(vcs.VersionCandidates), len(vcs.VersionIDs())))
	}

	return fmt.Sprintf("[%s]", strings.Join(lens, "; "))
}

func (candidates InputCandidates) Reduce(jobs JobSet) (InputMapping, bool) {
	newCandidates := candidates.pruneToCommonBuilds(jobs)

	for input, versionCandidates := range newCandidates {
		versionIDs := versionCandidates.VersionIDs()
		if len(versionIDs) == 1 {
			// already reduced
			continue
		}

		for _, id := range versionIDs {
			limitedToVersion := versionCandidates.ForVersion(id)

			inputCandidates := newCandidates[input]
			inputCandidates.VersionCandidates = limitedToVersion
			newCandidates[input] = inputCandidates

			mapping, ok := newCandidates.Reduce(jobs)
			if ok {
				// reduced via recursion, done
				return mapping, true
			}

			newCandidates[input] = versionCandidates
		}
	}

	mapping := InputMapping{}
	for _, versionCandidates := range newCandidates {
		versionIDs := versionCandidates.VersionIDs()
		if len(versionIDs) != 1 {
			// could not reduce
			return nil, false
		}

		jobIDs := versionCandidates.JobIDs()
		if !jobIDs.Equal(versionCandidates.Passed) {
			// did not satisfy all passed constraints
			return nil, false
		}

		mapping[versionCandidates.Input] = versionIDs[0]
	}

	return mapping, true
}

func (candidates InputCandidates) pruneToCommonBuilds(jobs JobSet) InputCandidates {
	newCandidates := make(InputCandidates, len(candidates))
	copy(newCandidates, candidates)

	for jobID, _ := range jobs {
		commonBuildIDs := newCandidates.commonBuildIDs(jobID)

		for i, versionCandidates := range newCandidates {
			inputCandidates := versionCandidates
			inputCandidates.VersionCandidates = versionCandidates.PruneVersionsOfOtherBuildIDs(jobID, commonBuildIDs)
			newCandidates[i] = inputCandidates
		}
	}

	return newCandidates
}

func (candidates InputCandidates) commonBuildIDs(jobID int) BuildSet {
	firstTick := true

	var commonBuildIDs BuildSet

	for _, set := range candidates {
		setBuildIDs := set.BuildIDs(jobID)
		if len(setBuildIDs) == 0 {
			continue
		}

		if firstTick {
			commonBuildIDs = setBuildIDs
		} else {
			commonBuildIDs = commonBuildIDs.Intersect(setBuildIDs)
		}

		firstTick = false
	}

	return commonBuildIDs
}
