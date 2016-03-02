package algorithm

import (
	"fmt"
	"sort"
)

type VersionCandidate struct {
	VersionID  int
	BuildID    int
	JobID      int
	CheckOrder int
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
	uniqueVersionIDCandidates := map[int]VersionCandidate{}
	uniqueCandidates := []VersionCandidate{}
	for candidate, _ := range candidates {
		if _, ok := uniqueVersionIDCandidates[candidate.VersionID]; !ok {
			uniqueVersionIDCandidates[candidate.VersionID] = candidate
			uniqueCandidates = append(uniqueCandidates, candidate)
		}
	}

	sorter := versionCandidatesSorter{
		VersionCandidates: uniqueCandidates,
	}

	sort.Sort(sort.Reverse(sorter))

	versionIDs := make([]int, len(uniqueCandidates))
	for i, candidate := range sorter.VersionCandidates {
		versionIDs[i] = candidate.VersionID
	}

	return versionIDs
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

type versionCandidatesSorter struct {
	VersionCandidates []VersionCandidate
}

func (s versionCandidatesSorter) Len() int {
	return len(s.VersionCandidates)
}

func (s versionCandidatesSorter) Swap(i, j int) {
	s.VersionCandidates[i], s.VersionCandidates[j] = s.VersionCandidates[j], s.VersionCandidates[i]
}

func (s versionCandidatesSorter) Less(i, j int) bool {
	return s.VersionCandidates[i].CheckOrder < s.VersionCandidates[j].CheckOrder
}
