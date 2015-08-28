package algorithm

type VersionsDB []BuildOutput

type BuildOutput struct {
	VersionID  int
	ResourceID int
	BuildID    int
	JobID      int
}

func (db VersionsDB) VersionsOfResourcePassedJobs(resourceID int, passed JobSet) VersionCandidates {
	if len(passed) == 0 {
		return db.versionsOfResource(resourceID)
	}

	var candidates VersionCandidates

	for jobID, _ := range passed {
		passedJob := db.versionsOfResourcePassedJob(resourceID, jobID)
		if candidates == nil {
			candidates = passedJob
		} else {
			candidates = candidates.IntersectByVersion(passedJob)
		}
	}

	return candidates
}

func (db VersionsDB) versionsOfResourcePassedJob(resourceID int, job int) VersionCandidates {
	var versions []VersionCandidate

	for _, output := range db {
		if output.ResourceID == resourceID && output.JobID == job {
			versions = append(versions, VersionCandidate{
				VersionID: output.VersionID,
				BuildID:   output.BuildID,
				JobID:     output.JobID,
			})
		}
	}

	return versions
}

func (db VersionsDB) versionsOfResource(resourceID int) VersionCandidates {
	// 0 = no job; a versioned resource unrelated to build outputs
	return db.versionsOfResourcePassedJob(resourceID, 0)
}
