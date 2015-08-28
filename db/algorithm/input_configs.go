package algorithm

type InputConfigs []InputConfig

type InputConfig struct {
	Name       string
	Passed     JobSet
	ResourceID int
}

func (configs InputConfigs) Resolve(db VersionsDB) (InputMapping, bool) {
	jobs := JobSet{}
	inputCandidates := InputCandidates{}
	for _, inputConfig := range configs {
		jobs = jobs.Union(inputConfig.Passed)

		candidateSet := db.VersionsOfResourcePassedJobs(
			inputConfig.ResourceID,
			inputConfig.Passed,
		)

		inputCandidates[inputConfig.Name] = InputVersionCandidates{
			VersionCandidates: candidateSet,
			Passed:            inputConfig.Passed,
		}
	}

	return inputCandidates.Reduce(jobs)
}
