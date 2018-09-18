package algorithm

import "fmt"

type VersionCandidate struct {
	VersionID  int
	BuildID    int
	JobID      int
	CheckOrder int
}

func (candidate VersionCandidate) String() string {
	return fmt.Sprintf("{v%d, j%db%d}", candidate.VersionID, candidate.JobID, candidate.BuildID)
}

type VersionCandidates struct {
	versions    Versions
	constraints Constraints
	buildIDs    map[int]BuildSet
}

func (candidates *VersionCandidates) Add(candidate VersionCandidate) {
	candidates.versions = candidates.versions.With(candidate)

	if candidate.JobID != 0 {
		if candidates.buildIDs == nil {
			candidates.buildIDs = map[int]BuildSet{}
		}

		builds, found := candidates.buildIDs[candidate.JobID]
		if !found {
			builds = BuildSet{}
			candidates.buildIDs[candidate.JobID] = builds
		}

		builds[candidate.BuildID] = struct{}{}
	}
}

func (candidates *VersionCandidates) Merge(version Version) {
	for jobID, otherBuilds := range version.passed {
		if candidates.buildIDs == nil {
			candidates.buildIDs = map[int]BuildSet{}
		}

		builds, found := candidates.buildIDs[jobID]
		if !found {
			builds = BuildSet{}
			candidates.buildIDs[jobID] = builds
		}

		for build := range otherBuilds {
			builds[build] = struct{}{}
		}
	}

	candidates.versions = candidates.versions.Merge(version)
}

func (candidates VersionCandidates) IsEmpty() bool {
	return len(candidates.versions) == 0
}

func (candidates VersionCandidates) Len() int {
	return len(candidates.versions)
}

func (candidates VersionCandidates) IntersectByVersion(other VersionCandidates) VersionCandidates {
	intersected := VersionCandidates{}

	for _, version := range candidates.versions {
		found := false
		for _, otherVersion := range other.versions {
			if otherVersion.id == version.id {
				found = true
				intersected.Merge(otherVersion)
				break
			}
		}

		if found {
			intersected.Merge(version)
		}
	}

	return intersected
}

func (candidates VersionCandidates) BuildIDs(jobID int) BuildSet {
	builds, found := candidates.buildIDs[jobID]
	if !found {
		builds = BuildSet{}
	}

	return builds
}

func (candidates VersionCandidates) PruneVersionsOfOtherBuildIDs(jobID int, buildIDs BuildSet) VersionCandidates {
	newCandidates := candidates
	newCandidates.constraints = newCandidates.constraints.And(func(v Version) bool {
		return v.PassedAny(jobID, buildIDs)
	})
	return newCandidates
}

type VersionsIter struct {
	offset      int
	versions    Versions
	constraints Constraints
}

func (iter *VersionsIter) Next() (int, bool) {
	for i := iter.offset; i < len(iter.versions); i++ {
		v := iter.versions[i]

		iter.offset++

		if !iter.constraints.Check(v) {
			continue
		}

		return v.id, true
	}

	return 0, false
}

func (iter *VersionsIter) Peek() (int, bool) {
	for i := iter.offset; i < len(iter.versions); i++ {
		v := iter.versions[i]

		if !iter.constraints.Check(v) {
			iter.offset++
			continue
		}

		return v.id, true
	}

	return 0, false
}

func (candidates VersionCandidates) VersionIDs() *VersionsIter {
	return &VersionsIter{
		versions:    candidates.versions,
		constraints: candidates.constraints,
	}
}

func (candidates VersionCandidates) ForVersion(versionID int) VersionCandidates {
	newCandidates := VersionCandidates{}
	for _, version := range candidates.versions {
		if version.id == versionID {
			newCandidates.Merge(version)
			break
		}
	}

	return newCandidates
}
