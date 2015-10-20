package algorithm

import (
	"fmt"
	"sort"
)

type VersionCandidate struct {
	VersionID int
	BuildID   int
	JobID     int
}

func (candidate VersionCandidate) String() string {
	return fmt.Sprintf("{v%d, j%db%d}", candidate.VersionID, candidate.JobID, candidate.BuildID)
}

type VersionCandidates map[VersionCandidate]struct{}

func (candidates VersionCandidates) IntersectByVersion(otherVersions VersionCandidates) VersionCandidates {
	intersected := VersionCandidates{}

	for version := range candidates {
		found := false
		for otherVersion := range otherVersions {
			if otherVersion.VersionID == version.VersionID {
				found = true
				intersected[otherVersion] = struct{}{}
			}
		}

		if found {
			intersected[version] = struct{}{}
		}
	}

	return intersected
}

func (candidates VersionCandidates) BuildIDs(jobID int) BuildSet {
	buildIDs := BuildSet{}
	for version := range candidates {
		if version.JobID == jobID {
			buildIDs[version.BuildID] = struct{}{}
		}
	}

	return buildIDs
}

func (candidates VersionCandidates) JobIDs() JobSet {
	ids := JobSet{}
	for version := range candidates {
		if version.JobID == 0 {
			continue
		}

		ids[version.JobID] = struct{}{}
	}

	return ids
}

func (candidates VersionCandidates) PruneVersionsOfOtherBuildIDs(jobID int, builds BuildSet) VersionCandidates {
	remaining := VersionCandidates{}
	for version := range candidates {
		if version.JobID != jobID || builds.Contains(version.BuildID) {
			remaining[version] = struct{}{}
		}
	}

	return remaining
}

func (candidates VersionCandidates) VersionIDs() []int {
	ids := map[int]struct{}{}
	for version := range candidates {
		ids[version.VersionID] = struct{}{}
	}

	sortedIDs := make([]int, len(ids))
	i := 0
	for id, _ := range ids {
		sortedIDs[i] = id
		i++
	}

	sort.Sort(sort.Reverse(sort.IntSlice(sortedIDs)))

	return sortedIDs
}

func (candidates VersionCandidates) ForVersion(versionID int) VersionCandidates {
	newCandidates := VersionCandidates{}
	for candidate := range candidates {
		if candidate.VersionID == versionID {
			newCandidates[candidate] = struct{}{}
		}
	}

	return newCandidates
}
