package db

import "github.com/concourse/atc"

type PlanConvertingConfigDB struct {
	NestedDB ConfigDB
}

func (db PlanConvertingConfigDB) GetConfig() (atc.Config, ConfigVersion, error) {
	config, version, err := db.NestedDB.GetConfig()
	if err != nil {
		return atc.Config{}, 0, err
	}

	return db.convertJobsToPlan(config), version, nil
}

func (db PlanConvertingConfigDB) SaveConfig(config atc.Config, version ConfigVersion) error {
	return db.NestedDB.SaveConfig(db.convertJobsToPlan(config), version)
}

func (db PlanConvertingConfigDB) convertJobsToPlan(config atc.Config) atc.Config {
	convertedJobs := make([]atc.JobConfig, len(config.Jobs))
	copy(convertedJobs, config.Jobs)

	for ji, job := range convertedJobs {
		if len(job.Plan) > 0 { // skip jobs already converted to plans
			continue
		}

		convertedSequence := atc.PlanSequence{}

		inputAggregates := make(atc.PlanSequence, len(job.InputConfigs))
		for ii, input := range job.InputConfigs {
			name := input.RawName
			resource := input.Resource
			if name == "" {
				name = input.Resource
				resource = ""
			}

			inputAggregates[ii] = atc.PlanConfig{
				Get:        name,
				Resource:   resource,
				RawTrigger: input.RawTrigger,
				Passed:     input.Passed,
				Params:     input.Params,
			}
		}

		if len(inputAggregates) > 0 {
			convertedSequence = append(convertedSequence, atc.PlanConfig{
				Aggregate: &inputAggregates,
			})
		}

		if job.TaskConfig != nil || job.TaskConfigPath != "" {
			convertedSequence = append(convertedSequence, atc.PlanConfig{
				Task:           "build", // default name
				TaskConfigPath: job.TaskConfigPath,
				TaskConfig:     job.TaskConfig,
				Privileged:     job.Privileged,
			})
		}

		outputAggregates := make(atc.PlanSequence, len(job.OutputConfigs))
		for oi, output := range job.OutputConfigs {
			var conditions *atc.Conditions
			if output.RawPerformOn != nil { // NOT len(0)
				conditionsCasted := atc.Conditions(output.RawPerformOn)
				conditions = &conditionsCasted
			}

			outputAggregates[oi] = atc.PlanConfig{
				Put:        output.Resource,
				Conditions: conditions,
				Params:     output.Params,
			}
		}

		if len(outputAggregates) > 0 {
			convertedSequence = append(convertedSequence, atc.PlanConfig{
				Aggregate: &outputAggregates,
			})
		}

		// zero-out old-style config so they're omitted from new payload
		convertedJobs[ji].InputConfigs = nil
		convertedJobs[ji].OutputConfigs = nil
		convertedJobs[ji].TaskConfigPath = ""
		convertedJobs[ji].TaskConfig = nil
		convertedJobs[ji].Privileged = false

		convertedJobs[ji].Plan = convertedSequence
	}

	config.Jobs = convertedJobs

	return config
}
