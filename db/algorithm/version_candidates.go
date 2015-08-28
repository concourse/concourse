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

type VersionCandidates []VersionCandidate

func (candidates VersionCandidates) IntersectByVersion(otherVersions VersionCandidates) VersionCandidates {
	var intersected VersionCandidates

	for _, version := range candidates {
		found := false
		for _, otherVersion := range otherVersions {
			if otherVersion.VersionID == version.VersionID {
				found = true
				intersected = append(intersected, otherVersion)
			}
		}

		if found {
			intersected = append(intersected, version)
		}
	}

	return intersected
}

func (candidates VersionCandidates) BuildIDs(jobID int) BuildSet {
	buildIDs := BuildSet{}
	for _, version := range candidates {
		if version.JobID == jobID {
			buildIDs[version.BuildID] = struct{}{}
		}
	}

	return buildIDs
}

func (candidates VersionCandidates) JobIDs() JobSet {
	ids := JobSet{}
	for _, version := range candidates {
		if version.JobID == 0 {
			continue
		}

		ids[version.JobID] = struct{}{}
	}

	return ids
}

func (candidates VersionCandidates) PruneVersionsOfOtherBuildIDs(jobID int, builds BuildSet) VersionCandidates {
	var remaining VersionCandidates
	for _, version := range candidates {
		if version.JobID != jobID || builds.Contains(version.BuildID) {
			remaining = append(remaining, version)
		}
	}

	return remaining
}

func (candidates VersionCandidates) VersionIDs() []int {
	ids := map[int]struct{}{}
	for _, version := range candidates {
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
	for _, candidate := range candidates {
		if candidate.VersionID == versionID {
			newCandidates = append(newCandidates, candidate)
		}
	}

	return newCandidates
}
