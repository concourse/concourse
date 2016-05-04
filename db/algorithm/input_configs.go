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

type MissingInputReasons map[string]string

func (configs InputConfigs) Resolve(db *VersionsDB) (InputMapping, bool, MissingInputReasons) {
	jobs := JobSet{}
	inputCandidates := InputCandidates{}
	missingInputReasons := MissingInputReasons{}

	for _, inputConfig := range configs {
		versionCandidates := VersionCandidates{}

		if len(inputConfig.Passed) == 0 {
			versionCandidates = db.AllVersionsForResource(inputConfig.ResourceID)
		} else {
			jobs = jobs.Union(inputConfig.Passed)

			versionCandidates = db.VersionsOfResourcePassedJobs(
				inputConfig.ResourceID,
				inputConfig.Passed,
			)
		}

		if len(versionCandidates) == 0 {
			missingInputReasons[inputConfig.Name] = "no versions available"
			return nil, false, missingInputReasons
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
