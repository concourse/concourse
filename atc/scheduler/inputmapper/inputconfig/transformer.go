package inputconfig

import (
	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/atc/db"
	"github.com/concourse/concourse/v5/atc/db/algorithm"
)

//go:generate counterfeiter . Transformer

type Transformer interface {
	TransformInputConfigs(db *algorithm.VersionsDB, jobName string, inputs []atc.JobInput) (algorithm.InputConfigs, error)
}

func NewTransformer(pipeline db.Pipeline) Transformer {
	return &transformer{pipeline: pipeline}
}

type transformer struct {
	pipeline db.Pipeline
}

func (i *transformer) TransformInputConfigs(db *algorithm.VersionsDB, jobName string, inputs []atc.JobInput) (algorithm.InputConfigs, error) {
	inputConfigs := algorithm.InputConfigs{}

	for _, input := range inputs {
		if input.Version == nil {
			input.Version = &atc.VersionConfig{Latest: true}
		}

		pinnedVersionID := 0
		if input.Version.Pinned != nil {
			resource, found, err := i.pipeline.Resource(input.Resource)
			if err != nil {
				return nil, err
			}

			if !found {
				continue
			}

			id, found, err := resource.ResourceConfigVersionID(input.Version.Pinned)
			if err != nil {
				return nil, err
			}

			if !found {
				continue
			}

			pinnedVersionID = id
		}

		jobs := algorithm.JobSet{}
		for _, passedJobName := range input.Passed {
			jobs[db.JobIDs[passedJobName]] = struct{}{}
		}

		inputConfigs = append(inputConfigs, algorithm.InputConfig{
			Name:            input.Name,
			UseEveryVersion: input.Version.Every,
			PinnedVersionID: pinnedVersionID,
			ResourceID:      db.ResourceIDs[input.Resource],
			Passed:          jobs,
			JobID:           db.JobIDs[jobName],
		})
	}

	return inputConfigs, nil
}
