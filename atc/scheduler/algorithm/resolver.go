package algorithm

import (
	"strings"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/tracing"
)

type NameToIDMap map[string]int

func (cfgs InputConfigs) String() string {
	if !tracing.Configured {
		return ""
	}

	names := make([]string, len(cfgs))
	for i, cfg := range cfgs {
		names[i] = cfg.Name
	}

	return strings.Join(names, ",")
}

type relatedInputConfigs struct {
	passedJobs   map[int]bool
	inputConfigs InputConfigs
}

func constructResolvers(
	versions db.VersionsDB,
	inputConfigs InputConfigs,
) ([]Resolver, error) {
	resolvers := []Resolver{}
	inputConfigsWithPassed := InputConfigs{}
	for _, inputConfig := range inputConfigs {
		if len(inputConfig.Passed) == 0 {
			if inputConfig.PinnedVersion != nil {
				resolvers = append(resolvers, NewPinnedResolver(versions, inputConfig))
			} else {
				resolvers = append(resolvers, NewIndividualResolver(versions, inputConfig))
			}
		} else {
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
