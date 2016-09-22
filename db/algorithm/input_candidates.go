package algorithm

import (
	"fmt"
	"strings"
)

type InputCandidates []InputVersionCandidates

type ResolvedInputs map[string]int

type InputVersionCandidates struct {
	Input                 string
	Passed                JobSet
	UseEveryVersion       bool
	PinnedVersionID       int
	ExistingBuildResolver *ExistingBuildResolver
	usingEveryVersion     *bool

	VersionCandidates
}

func (inputVersionCandidates InputVersionCandidates) IsNext(version int, versionIDs *VersionsIter) bool {
	if !inputVersionCandidates.UsingEveryVersion() {
		return true
	}

	if inputVersionCandidates.ExistingBuildResolver.ExistsForVersion(version) {
		return true
	}

	next, hasNext := versionIDs.Peek()
	return !hasNext ||
		inputVersionCandidates.ExistingBuildResolver.ExistsForVersion(next)
}

func (inputVersionCandidates InputVersionCandidates) UsingEveryVersion() bool {
	if inputVersionCandidates.usingEveryVersion == nil {
		usingEveryVersion := inputVersionCandidates.UseEveryVersion &&
			inputVersionCandidates.ExistingBuildResolver.Exists()
		inputVersionCandidates.usingEveryVersion = &usingEveryVersion
	}

	return *inputVersionCandidates.usingEveryVersion
}

func (candidates InputCandidates) String() string {
	lens := []string{}
	for _, vcs := range candidates {
		lens = append(lens, fmt.Sprintf("%s (%d versions)", vcs.Input, vcs.VersionCandidates.Len()))
	}

	return fmt.Sprintf("[%s]", strings.Join(lens, "; "))
}

func (candidates InputCandidates) Reduce(depth int, jobs JobSet) (ResolvedInputs, bool) {
	newInputCandidates := candidates.pruneToCommonBuilds(jobs)

	for i, inputVersionCandidates := range newInputCandidates {
		if inputVersionCandidates.Len() == 1 {
			// already reduced
			continue
		}

		if inputVersionCandidates.PinnedVersionID != 0 {
			newInputCandidates.Pin(i, inputVersionCandidates.PinnedVersionID)
			continue
		}

		versionIDs := inputVersionCandidates.VersionIDs()

		iteration := 0

		for {
			id, ok := versionIDs.Next()
			if !ok {
				// exhaused available versions
				return nil, false
			}

			iteration++

			newInputCandidates.Pin(i, id)

			mapping, ok := newInputCandidates.Reduce(depth+1, jobs)
			if ok && inputVersionCandidates.IsNext(id, versionIDs) {
				return mapping, true
			}

			newInputCandidates.Unpin(i, inputVersionCandidates)
		}
	}

	resolved := ResolvedInputs{}

	for _, inputVersionCandidates := range newInputCandidates {
		vids := inputVersionCandidates.VersionIDs()

		vid, ok := vids.Next()
		if !ok {
			return nil, false
		}

		resolved[inputVersionCandidates.Input] = vid
	}

	return resolved, true
}

func (candidates InputCandidates) Pin(input int, version int) {
	limitedToVersion := candidates[input].ForVersion(version)

	inputCandidates := candidates[input]
	inputCandidates.VersionCandidates = limitedToVersion
	candidates[input] = inputCandidates
}

func (candidates InputCandidates) Unpin(input int, inputCandidates InputVersionCandidates) {
	candidates[input] = inputCandidates
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

	commonBuildIDs := BuildSet{}

	for _, set := range candidates {
		setBuildIDs := set.BuildIDs(jobID)
		if len(setBuildIDs) == 0 {
			continue
		}

		if firstTick {
			for id := range setBuildIDs {
				commonBuildIDs[id] = struct{}{}
			}
		} else {
			for id := range commonBuildIDs {
				_, found := setBuildIDs[id]
				if !found {
					delete(commonBuildIDs, id)
				}
			}
		}

		firstTick = false
	}

	return commonBuildIDs
}
