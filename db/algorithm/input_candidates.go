package algorithm

import (
	"fmt"
	"strings"
)

type InputCandidates []InputVersionCandidates

type InputVersionCandidates struct {
	Input                 string
	Passed                JobSet
	Version               string
	ExistingBuildResolver *ExistingBuildResolver
	usingEveryVersion     *bool

	VersionCandidates
}

func (inputVersionCandidates InputVersionCandidates) UseEveryVersion() bool {
	if inputVersionCandidates.usingEveryVersion == nil {
		usingEveryVersion := inputVersionCandidates.Version == VersionEvery &&
			inputVersionCandidates.ExistingBuildResolver.Exists()
		inputVersionCandidates.usingEveryVersion = &usingEveryVersion
	}

	return *inputVersionCandidates.usingEveryVersion
}

const VersionEvery = "every"

func (candidates InputCandidates) String() string {
	lens := []string{}
	for _, vcs := range candidates {
		lens = append(lens, fmt.Sprintf("%s (%d candidates for %d versions)", vcs.Input, len(vcs.VersionCandidates), len(vcs.VersionIDs())))
	}

	return fmt.Sprintf("[%s]", strings.Join(lens, "; "))
}

func (candidates InputCandidates) Reduce(jobs JobSet) (InputMapping, bool) {
	return candidates.reduce(jobs, nil)
}

func (candidates InputCandidates) reduce(jobs JobSet, lastSatisfiedMapping InputMapping) (InputMapping, bool) {
	newCandidates := candidates.pruneToCommonBuilds(jobs)

	for input, versionCandidates := range newCandidates {
		versionIDs := versionCandidates.VersionIDs()
		if len(versionIDs) == 1 {
			// already reduced
			continue
		}

		usingEveryVersion := versionCandidates.UseEveryVersion()

		for i, id := range versionIDs {
			buildForPreviousOrCurrentVersionExists := func() bool {
				return versionCandidates.ExistingBuildResolver.ExistsForVersion(id) ||
					i == len(versionIDs)-1 ||
					versionCandidates.ExistingBuildResolver.ExistsForVersion(versionIDs[i+1])
			}

			limitedToVersion := versionCandidates.ForVersion(id)

			inputCandidates := newCandidates[input]
			inputCandidates.VersionCandidates = limitedToVersion
			newCandidates[input] = inputCandidates

			mapping, ok := newCandidates.reduce(jobs, lastSatisfiedMapping)
			if ok {
				lastSatisfiedMapping = mapping
				if !usingEveryVersion || buildForPreviousOrCurrentVersionExists() {
					return mapping, true
				}
			} else {
				if usingEveryVersion && (lastSatisfiedMapping != nil || buildForPreviousOrCurrentVersionExists()) {
					return lastSatisfiedMapping, true
				}
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
