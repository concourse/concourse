package algorithm

import "github.com/concourse/concourse/atc/db"

type InputCandidates []InputVersionCandidates

type ResolvedInputs map[string]int

type InputVersionCandidates struct {
	Input           string
	Passed          db.JobSet
	UseEveryVersion bool
	PinnedVersionID int

	VersionCandidates

	hasUsedResource *bool
}

func (candidates InputCandidates) Reduce(depth int, jobs db.JobSet) (ResolvedInputs, bool, error) {
	newInputCandidates := candidates.pruneToCommonBuilds(jobs)

	for i, inputVersionCandidates := range newInputCandidates {
		if inputVersionCandidates.Pinned() {
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
			id, ok, err := versionIDs.Next()
			if err != nil {
				return nil, false, err
			}

			if !ok {
				// exhaused available versions
				return nil, false, nil
			}

			iteration++

			newInputCandidates.Pin(i, id)

			mapping, ok, err := newInputCandidates.Reduce(depth+1, jobs)
			if err != nil {
				return nil, false, err
			}

			if ok {
				return mapping, true, nil
			}

			newInputCandidates.Unpin(i, inputVersionCandidates)
		}
	}

	resolved := ResolvedInputs{}

	for _, inputVersionCandidates := range newInputCandidates {
		vids := inputVersionCandidates.VersionIDs()

		vid, ok, err := vids.Next()
		if err != nil {
			return nil, false, err
		}

		if !ok {
			return nil, false, nil
		}

		resolved[inputVersionCandidates.Input] = vid
	}

	return resolved, true, nil
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

func (candidates InputCandidates) pruneToCommonBuilds(jobs db.JobSet) InputCandidates {
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
