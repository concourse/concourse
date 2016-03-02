package algorithm

import "time"

type VersionsDB struct {
	ResourceVersions []ResourceVersion
	BuildOutputs     []BuildOutput
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

func (db VersionsDB) VersionsOfResourcePassedJobs(resourceID int, passed JobSet) VersionCandidates {
	if len(passed) == 0 {
		return db.versionsOfResource(resourceID)
	}

	firstTick := true

	var candidates VersionCandidates

	for jobID, _ := range passed {
		passedJob := db.versionsOfResourcePassedJob(resourceID, jobID)
		if firstTick {
			candidates = passedJob
		} else {
			candidates = candidates.IntersectByVersion(passedJob)
		}

		firstTick = false
	}

	return candidates
}

func (db VersionsDB) versionsOfResourcePassedJob(resourceID int, job int) VersionCandidates {
	versions := VersionCandidates{}

	for _, output := range db.BuildOutputs {
		if output.ResourceID == resourceID && output.JobID == job {
			versions[VersionCandidate{
				VersionID:  output.VersionID,
				BuildID:    output.BuildID,
				JobID:      output.JobID,
				CheckOrder: output.CheckOrder,
			}] = struct{}{}
		}
	}

	return versions
}

func (db VersionsDB) versionsOfResource(resourceID int) VersionCandidates {
	versions := VersionCandidates{}

	for _, output := range db.ResourceVersions {
		if output.ResourceID == resourceID {
			versions[VersionCandidate{
				VersionID:  output.VersionID,
				CheckOrder: output.CheckOrder,
			}] = struct{}{}
		}
	}

	return versions
}
