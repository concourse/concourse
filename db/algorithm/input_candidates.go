package algorithm

type InputCandidates map[string]InputVersionCandidates

type InputVersionCandidates struct {
	VersionCandidates
	Passed JobSet
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
	for input, versionCandidates := range newCandidates {
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

		mapping[input] = versionIDs[0]
	}

	return mapping, true
}

func (candidates InputCandidates) pruneToCommonBuilds(jobs JobSet) InputCandidates {
	newCandidates := InputCandidates{}
	for input, versions := range candidates {
		newCandidates[input] = versions
	}

	for jobID, _ := range jobs {
		commonBuildIDs := newCandidates.commonBuildIDs(jobID)

		for input, versionCandidates := range newCandidates {
			inputCandidates := versionCandidates
			inputCandidates.VersionCandidates = versionCandidates.PruneVersionsOfOtherBuildIDs(jobID, commonBuildIDs)
			newCandidates[input] = inputCandidates
		}
	}

	return newCandidates
}

func (candidates InputCandidates) commonBuildIDs(jobID int) BuildSet {
	var commonBuildIDs BuildSet

	for _, set := range candidates {
		setBuildIDs := set.BuildIDs(jobID)
		if len(setBuildIDs) == 0 {
			continue
		}

		if commonBuildIDs == nil {
			commonBuildIDs = setBuildIDs
		} else {
			commonBuildIDs = commonBuildIDs.Intersect(setBuildIDs)
		}
	}

	return commonBuildIDs
}
