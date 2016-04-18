package algorithm

type ExistingBuildResolver struct {
	BuildInputs []BuildInput
	JobID       int
	ResourceID  int
}

func (r *ExistingBuildResolver) Exists() bool {
	for _, buildInput := range r.BuildInputs {
		if buildInput.JobID == r.JobID && buildInput.ResourceID == r.ResourceID {
			return true
		}
	}

	return false
}

func (r *ExistingBuildResolver) ExistsForVersion(versionID int) bool {
	for _, buildInput := range r.BuildInputs {
		if buildInput.JobID == r.JobID && buildInput.ResourceID == r.ResourceID {
			if buildInput.VersionID == versionID {
				return true
			}
		}
	}

	return false
}
