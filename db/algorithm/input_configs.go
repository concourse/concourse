package algorithm

import "sort"

type InputConfigs []InputConfig

type InputConfig struct {
	Name       string
	JobName    string
	Passed     JobSet
	Version    string
	ResourceID int
	JobID      int
}

func (configs InputConfigs) Resolve(db *VersionsDB) (InputMapping, bool) {
	jobs := JobSet{}
	inputCandidates := InputCandidates{}
	for _, inputConfig := range configs {
		candidateSet := VersionCandidates{}

		if len(inputConfig.Passed) == 0 {
			candidateSet = db.AllVersionsForResource(inputConfig.ResourceID)
		} else {
			jobs = jobs.Union(inputConfig.Passed)

			candidateSet = db.VersionsOfResourcePassedJobs(
				inputConfig.ResourceID,
				inputConfig.Passed,
			)
		}

		if len(candidateSet) == 0 {
			return nil, false
		}

		existingBuildResolver := &ExistingBuildResolver{
			BuildInputs: db.BuildInputs,
			JobID:       inputConfig.JobID,
			ResourceID:  inputConfig.ResourceID,
		}

		inputCandidates = append(inputCandidates, InputVersionCandidates{
			Input:                 inputConfig.Name,
			Passed:                inputConfig.Passed,
			Version:               inputConfig.Version,
			VersionCandidates:     candidateSet,
			ExistingBuildResolver: existingBuildResolver,
		})
	}

	sort.Sort(byTotalVersions(inputCandidates))

	return inputCandidates.Reduce(jobs)
}

type byTotalVersions InputCandidates

func (candidates byTotalVersions) Len() int { return len(candidates) }

func (candidates byTotalVersions) Swap(i int, j int) {
	candidates[i], candidates[j] = candidates[j], candidates[i]
}

func (candidates byTotalVersions) Less(i int, j int) bool {
	return len(candidates[i].VersionIDs()) > len(candidates[j].VersionIDs())
}
