package algorithm

import (
	"errors"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

type InputConfigs []InputConfig

type InputConfig struct {
	Name            string
	Passed          db.JobSet
	UseEveryVersion bool
	PinnedVersion   atc.Version
	ResourceID      int
	JobID           int
}

func (a *Algorithm) CreateInputConfigs(
	jobID int,
	jobInputs []atc.JobInput,
	resources db.Resources,
	relatedJobs NameToIDMap,
) (InputConfigs, error) {

	inputConfigs := InputConfigs{}
	for _, input := range jobInputs {
		resource, found := resources.Lookup(input.Resource)
		if !found {
			return nil, errors.New("input resource not found")
		}

		inputConfig := InputConfig{
			Name:       input.Name,
			ResourceID: resource.ID(),
			JobID:      jobID,
		}

		var pinnedVersion atc.Version
		if resource.CurrentPinnedVersion() != nil {
			pinnedVersion = resource.CurrentPinnedVersion()
		}

		if input.Version != nil && input.Version.Pinned != nil {
			pinnedVersion = input.Version.Pinned
		}

		inputConfig.PinnedVersion = pinnedVersion

		if inputConfig.PinnedVersion == nil {
			if input.Version != nil {
				inputConfig.UseEveryVersion = input.Version.Every
			}
		}

		jobs := db.JobSet{}
		for _, passedJobName := range input.Passed {
			jobID := relatedJobs[passedJobName]
			jobs[jobID] = true
		}

		inputConfig.Passed = jobs
		inputConfigs = append(inputConfigs, inputConfig)
	}

	return inputConfigs, nil
}
