package algorithm

import "sort"

type Version struct {
	id     int
	order  int
	passed map[int]BuildSet
}

func NewVersion(candidate VersionCandidate) Version {
	v := Version{
		id:     candidate.VersionID,
		order:  candidate.CheckOrder,
		passed: map[int]BuildSet{},
	}

	if candidate.JobID != 0 {
		v.passed[candidate.JobID] = BuildSet{candidate.BuildID: struct{}{}}
	}

	return v
}

func (v Version) PassedAny(jobID int, builds BuildSet) bool {
	bs, found := v.passed[jobID]
	if !found {
		return true
	}

	return bs.Overlaps(builds)
}

type Versions []Version

func (vs Versions) With(candidate VersionCandidate) Versions {
	i := sort.Search(len(vs), func(i int) bool {
		return vs[i].order <= candidate.CheckOrder
	})
	if i == len(vs) {
		return append(vs, NewVersion(candidate))
	}

	if vs[i].id != candidate.VersionID {
		vs = append(vs, Version{})
		copy(vs[i+1:], vs[i:])
		vs[i] = NewVersion(candidate)
	} else if candidate.JobID != 0 {
		builds, found := vs[i].passed[candidate.JobID]
		if !found {
			builds = BuildSet{}
			vs[i].passed[candidate.JobID] = builds
		}

		builds[candidate.BuildID] = struct{}{}
	}

	return vs
}

func (vs Versions) Merge(v Version) Versions {
	i := sort.Search(len(vs), func(i int) bool {
		return vs[i].order <= v.order
	})
	if i == len(vs) {
		return append(vs, v)
	}

	if vs[i].id != v.id {
		vs = append(vs, Version{})
		copy(vs[i+1:], vs[i:])
		vs[i] = v
	} else {
		for jobID, vbuilds := range v.passed {
			builds, found := vs[i].passed[jobID]
			if !found {
				vs[i].passed[jobID] = vbuilds
				continue
			}

			for vbuild := range vbuilds {
				builds[vbuild] = struct{}{}
			}
		}
	}

	return vs
}
