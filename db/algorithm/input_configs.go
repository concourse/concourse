package algorithm

import "sort"

type InputConfigs []InputConfig

type Version struct {
	Every  bool
	Pinned map[string]string
}

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
			versionCandidates = db.AllVersionsForResource(inputConfig.ResourceID)

			if len(versionCandidates) == 0 {
				return nil, false
			}
		} else {
			jobs = jobs.Union(inputConfig.Passed)

			versionCandidates = db.VersionsOfResourcePassedJobs(
				inputConfig.ResourceID,
				inputConfig.Passed,
			)

			if len(versionCandidates) == 0 {
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

	sort.Sort(byTotalVersions(inputCandidates))

	basicMapping, ok := inputCandidates.Reduce(jobs)
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

type byTotalVersions InputCandidates

func (candidates byTotalVersions) Len() int { return len(candidates) }

func (candidates byTotalVersions) Swap(i int, j int) {
	candidates[i], candidates[j] = candidates[j], candidates[i]
}

func (candidates byTotalVersions) Less(i int, j int) bool {
	return len(candidates[i].VersionIDs()) > len(candidates[j].VersionIDs())
}
