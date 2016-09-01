package algorithm

import (
	"fmt"
	"strings"
)

type InputCandidates []InputVersionCandidates

type InputVersionCandidates struct {
	Input                 string
	Passed                JobSet
	UseEveryVersion       bool
	PinnedVersionID       int
	ExistingBuildResolver *ExistingBuildResolver
	usingEveryVersion     *bool

	VersionCandidates
}

func (inputVersionCandidates InputVersionCandidates) UsingEveryVersion() bool {
	if inputVersionCandidates.usingEveryVersion == nil {
		usingEveryVersion := inputVersionCandidates.UseEveryVersion &&
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

func (candidates InputCandidates) Reduce(jobs JobSet) (map[string]int, bool) {
	return candidates.reduce(jobs)
}

func (candidates InputCandidates) reduce(jobs JobSet) (map[string]int, bool) {
	newInputCandidates := candidates.pruneToCommonBuilds(jobs)

	var lastSatisfiedMapping map[string]int
	for i, inputVersionCandidates := range newInputCandidates {
		versionIDs := inputVersionCandidates.VersionIDs()
		switch {
		case len(versionIDs) == 1:
			// already reduced
			continue
		case inputVersionCandidates.PinnedVersionID != 0:
			limitedToVersion := inputVersionCandidates.ForVersion(inputVersionCandidates.PinnedVersionID)

			inputCandidates := newInputCandidates[i]
			inputCandidates.VersionCandidates = limitedToVersion
			newInputCandidates[i] = inputCandidates
		default:
			usingEveryVersion := inputVersionCandidates.UsingEveryVersion()

			for j, id := range versionIDs {
				buildForPreviousOrCurrentVersionExists := func() bool {
					return inputVersionCandidates.ExistingBuildResolver.ExistsForVersion(id) ||
						j == len(versionIDs)-1 ||
						inputVersionCandidates.ExistingBuildResolver.ExistsForVersion(versionIDs[j+1])
				}

				limitedToVersion := inputVersionCandidates.ForVersion(id)

				inputCandidates := newInputCandidates[i]
				inputCandidates.VersionCandidates = limitedToVersion
				newInputCandidates[i] = inputCandidates

				mapping, ok := newInputCandidates.reduce(jobs)
				if ok {
					lastSatisfiedMapping = mapping

					if !usingEveryVersion || buildForPreviousOrCurrentVersionExists() {
						// when using every version return last option anyway
						return mapping, true
					}

				} else if usingEveryVersion && lastSatisfiedMapping != nil && buildForPreviousOrCurrentVersionExists() {
					// when using every version checked all options from latest version
					// down to to the last build, returning the earliest that satisfied
					return lastSatisfiedMapping, true
				}

				newInputCandidates[i] = inputVersionCandidates
			}
		}
	}

	mapping := map[string]int{}

	for _, inputVersionCandidates := range newInputCandidates {
		versionIDs := inputVersionCandidates.VersionIDs()
		if len(versionIDs) != 1 {
			// could not reduce
			return nil, false
		}

		jobIDs := inputVersionCandidates.JobIDs()
		if !jobIDs.Equal(inputVersionCandidates.Passed) {
			// did not satisfy all passed constraints
			return nil, false
		}

		mapping[inputVersionCandidates.Input] = versionIDs[0]
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
