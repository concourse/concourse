package algorithm

import "sort"

type InputConfigs []InputConfig

type InputConfig struct {
	Name       string
	Passed     JobSet
	ResourceID int
}

func (configs InputConfigs) Resolve(db *VersionsDB) (InputMapping, bool) {
	jobs := JobSet{}
	inputCandidates := InputCandidates{}
	for _, inputConfig := range configs {
		jobs = jobs.Union(inputConfig.Passed)

		candidateSet := db.VersionsOfResourcePassedJobs(
			inputConfig.ResourceID,
			inputConfig.Passed,
		)

		if len(candidateSet) == 0 {
			return nil, false
		}

		inputCandidates = append(inputCandidates, InputVersionCandidates{
			Input:  inputConfig.Name,
			Passed: inputConfig.Passed,

			VersionCandidates: candidateSet,
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
