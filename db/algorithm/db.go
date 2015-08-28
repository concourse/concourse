package algorithm

type VersionsDB struct {
	ResourceVersions []ResourceVersion
	BuildOutputs     []BuildOutput
	JobIDs           map[string]int
	ResourceIDs      map[string]int
}

type ResourceVersion struct {
	VersionID  int
	ResourceID int
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

	for _, output := range db.BuildOutputs {
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
	var versions []VersionCandidate

	for _, output := range db.ResourceVersions {
		if output.ResourceID == resourceID {
			versions = append(versions, VersionCandidate{
				VersionID: output.VersionID,
			})
		}
	}

	return versions
}
