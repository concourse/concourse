package algorithm

import (
	"github.com/concourse/concourse/atc/db"
)

type NameToIDMap map[string]int

type relatedInputConfigs struct {
	passedJobs   map[int]bool
	inputConfigs db.InputConfigs
}

func constructResolvers(
	versions db.VersionsDB,
	inputs db.InputConfigs,
) ([]Resolver, error) {
	resolvers := []Resolver{}
	inputConfigsWithPassed := db.InputConfigs{}
	for _, input := range inputs {
		if len(input.Passed) == 0 {
			if input.PinnedVersion != nil {
				resolvers = append(resolvers, NewPinnedResolver(versions, input))
			} else {
				resolvers = append(resolvers, NewIndividualResolver(versions, input))
			}
		} else {
			inputConfigsWithPassed = append(inputConfigsWithPassed, input)
		}
	}

	groupedInputConfigs := groupInputsConfigsByPassedJobs(inputConfigsWithPassed)

	for _, group := range groupedInputConfigs {
		resolvers = append(resolvers, NewGroupResolver(versions, group.inputConfigs))
	}

	return resolvers, nil
}

func groupInputsConfigsByPassedJobs(passedInputConfigs db.InputConfigs) []relatedInputConfigs {
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
				inputConfigs: db.InputConfigs{inputConfig},
			})
		}
	}

	return groupedPassedInputConfigs
}
