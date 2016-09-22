package algorithm

type InputConfigs []InputConfig

type InputConfig struct {
	Name            string
	JobName         string
	Passed          JobSet
	UseEveryVersion bool
	PinnedVersionID int
	ResourceID      int
	JobID           int
}

func (configs InputConfigs) Resolve(db *VersionsDB) (InputMapping, bool) {
	jobs := JobSet{}
	inputCandidates := InputCandidates{}

	for _, inputConfig := range configs {
		versionCandidates := VersionCandidates{}

		if len(inputConfig.Passed) == 0 {
			if inputConfig.UseEveryVersion {
				versionCandidates = db.AllVersionsOfResource(inputConfig.ResourceID)
			} else {
				var versionCandidate VersionCandidate
				var found bool

				if inputConfig.PinnedVersionID != 0 {
					versionCandidate, found = db.FindVersionOfResource(inputConfig.ResourceID, inputConfig.PinnedVersionID)
				} else {
					versionCandidate, found = db.LatestVersionOfResource(inputConfig.ResourceID)
				}

				if found {
					versionCandidates.Add(versionCandidate)
				}
			}

			if versionCandidates.IsEmpty() {
				return nil, false
			}
		} else {
			jobs = jobs.Union(inputConfig.Passed)

			versionCandidates = db.VersionsOfResourcePassedJobs(
				inputConfig.ResourceID,
				inputConfig.Passed,
			)

			if versionCandidates.IsEmpty() {
				return nil, false
			}
		}

		existingBuildResolver := &ExistingBuildResolver{
			BuildInputs: db.BuildInputs,
			JobID:       inputConfig.JobID,
			ResourceID:  inputConfig.ResourceID,
		}

		inputCandidates = append(inputCandidates, InputVersionCandidates{
			Input:                 inputConfig.Name,
			Passed:                inputConfig.Passed,
			UseEveryVersion:       inputConfig.UseEveryVersion,
			PinnedVersionID:       inputConfig.PinnedVersionID,
			VersionCandidates:     versionCandidates,
			ExistingBuildResolver: existingBuildResolver,
		})
	}

	basicMapping, ok := inputCandidates.Reduce(0, jobs)
	if !ok {
		return nil, false
	}

	mapping := InputMapping{}
	for _, inputConfig := range configs {
		inputName := inputConfig.Name
		inputVersionID := basicMapping[inputName]
		firstOccurrence := db.IsVersionFirstOccurrence(inputVersionID, inputConfig.JobID, inputName)
		mapping[inputName] = InputVersion{
			VersionID:       inputVersionID,
			FirstOccurrence: firstOccurrence,
		}
	}

	return mapping, true
}
