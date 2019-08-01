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

type relatedInputConfigs struct {
	passedJobs   map[int]bool
	inputConfigs InputConfigs
}

func constructResolvers(
	versions *db.VersionsDB,
	job db.Job,
	resources db.Resources,
) ([]Resolver, error) {
	resolvers := []Resolver{}
	inputConfigsWithPassed := InputConfigs{}
	for _, input := range job.Config().Inputs() {
		resource, found := resources.Lookup(input.Resource)
		if !found {
			return nil, errors.New("input resource not found")
		}

		inputConfig := InputConfig{
			Name:       input.Name,
			ResourceID: versions.ResourceIDs[input.Resource],
			JobID:      job.ID(),
		}

		var pinnedVersion atc.Version
		if resource.CurrentPinnedVersion() != nil {
			pinnedVersion = resource.CurrentPinnedVersion()
		}

		if input.Version != nil {
			inputConfig.UseEveryVersion = input.Version.Every

			if input.Version.Pinned != nil {
				pinnedVersion = input.Version.Pinned
			}
		}

		inputConfig.PinnedVersion = pinnedVersion

		if len(input.Passed) == 0 {
			if inputConfig.PinnedVersion != nil {
				resolvers = append(resolvers, NewPinnedResolver(versions, inputConfig))
			} else {
				resolvers = append(resolvers, NewIndividualResolver(versions, inputConfig))
			}
		} else {
			jobs := db.JobSet{}
			for _, passedJobName := range input.Passed {
				jobID := versions.JobIDs[passedJobName]
				jobs[jobID] = true
			}

			inputConfig.Passed = jobs
			inputConfigsWithPassed = append(inputConfigsWithPassed, inputConfig)
		}
	}

	groupedInputConfigs := groupInputsConfigsByPassedJobs(inputConfigsWithPassed)

	for _, group := range groupedInputConfigs {
		resolvers = append(resolvers, NewGroupResolver(versions, group.inputConfigs))
	}

	return resolvers, nil
}

func groupInputsConfigsByPassedJobs(passedInputConfigs InputConfigs) []relatedInputConfigs {
	groupedPassedInputConfigs := []relatedInputConfigs{}
	for _, inputConfig := range passedInputConfigs {
		var index int
		var found bool

		for passedJob, _ := range inputConfig.Passed {
			for groupIndex, group := range groupedPassedInputConfigs {
				if group.passedJobs[passedJob] {
					found = true
					index = groupIndex
				}
			}
		}

		if found {
			groupedPassedInputConfigs[index].inputConfigs = append(groupedPassedInputConfigs[index].inputConfigs, inputConfig)

			for inputPassedJob, _ := range inputConfig.Passed {
				if !groupedPassedInputConfigs[index].passedJobs[inputPassedJob] {
					groupedPassedInputConfigs[index].passedJobs[inputPassedJob] = true
				}
			}
		} else {
			passedJobs := map[int]bool{}
			for jobID, _ := range inputConfig.Passed {
				passedJobs[jobID] = true
			}

			groupedPassedInputConfigs = append(groupedPassedInputConfigs, relatedInputConfigs{
				passedJobs:   passedJobs,
				inputConfigs: InputConfigs{inputConfig},
			})
		}
	}

	return groupedPassedInputConfigs
}
