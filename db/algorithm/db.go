package algorithm

import "time"

type VersionsDB struct {
	ResourceVersions []ResourceVersion
	BuildOutputs     []BuildOutput
	BuildInputs      []BuildInput
	JobIDs           map[string]int
	ResourceIDs      map[string]int
	CachedAt         time.Time
}

type ResourceVersion struct {
	VersionID  int
	ResourceID int
	CheckOrder int
}

type BuildOutput struct {
	ResourceVersion
	BuildID int
	JobID   int
}

type BuildInput struct {
	ResourceVersion
	BuildID   int
	JobID     int
	InputName string
}

func (db VersionsDB) IsVersionFirstOccurrence(versionID int, jobID int, inputName string) bool {
	for _, buildInput := range db.BuildInputs {
		if buildInput.VersionID == versionID &&
			buildInput.JobID == jobID &&
			buildInput.InputName == inputName {
			return false
		}
	}
	return true
}

func (db VersionsDB) AllVersionsForResource(resourceID int) VersionCandidates {
	candidates := VersionCandidates{}
	for _, output := range db.ResourceVersions {
		if output.ResourceID == resourceID {
			candidates[VersionCandidate{
				VersionID:  output.VersionID,
				CheckOrder: output.CheckOrder,
			}] = struct{}{}
		}
	}

	return candidates
}

func (db VersionsDB) VersionsOfResourcePassedJobs(resourceID int, passed JobSet) VersionCandidates {
	candidates := VersionCandidates{}

	firstTick := true
	for jobID, _ := range passed {
		versions := VersionCandidates{}

		for _, output := range db.BuildOutputs {
			if output.ResourceID == resourceID && output.JobID == jobID {
				versions[VersionCandidate{
					VersionID:  output.VersionID,
					BuildID:    output.BuildID,
					JobID:      output.JobID,
					CheckOrder: output.CheckOrder,
				}] = struct{}{}
			}
		}

		if firstTick {
			candidates = versions
			firstTick = false
		} else {
			candidates = candidates.IntersectByVersion(versions)
		}
	}

	return candidates
}
