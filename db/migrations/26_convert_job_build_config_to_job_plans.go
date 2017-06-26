package migrations

import (
	"database/sql"
	"encoding/json"

	"github.com/concourse/atc/db/migration"
	internal "github.com/concourse/atc/db/migrations/internal/26"
)

func ConvertJobBuildConfigToJobPlans(tx migration.LimitedTx) error {
	var configPayload []byte

	err := tx.QueryRow(`
    SELECT config
    FROM config
  `).Scan(&configPayload)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}

		return err
	}

	var config internal.Config
	err = json.Unmarshal(configPayload, &config)
	if err != nil {
		return err
	}

	for ji, job := range config.Jobs {
		if len(job.Plan) > 0 { // skip jobs already converted to plans
			continue
		}

		convertedSequence := internal.PlanSequence{}

		inputAggregates := make(internal.PlanSequence, len(job.InputConfigs))
		for ii, input := range job.InputConfigs {
			name := input.RawName
			resource := input.Resource
			if name == "" {
				name = input.Resource
				resource = ""
			}

			inputAggregates[ii] = internal.PlanConfig{
				Get:        name,
				Resource:   resource,
				RawTrigger: input.RawTrigger,
				Passed:     input.Passed,
				Params:     input.Params,
			}
		}

		if len(inputAggregates) > 0 {
			convertedSequence = append(convertedSequence, internal.PlanConfig{Aggregate: &inputAggregates})
		}

		if job.TaskConfig != nil || job.TaskConfigPath != "" {
			convertedSequence = append(convertedSequence, internal.PlanConfig{
				Task:           "build", // default name
				TaskConfigPath: job.TaskConfigPath,
				TaskConfig:     job.TaskConfig,
				Privileged:     job.Privileged,
			})
		}

		outputAggregates := make(internal.PlanSequence, len(job.OutputConfigs))
		for oi, output := range job.OutputConfigs {
			var conditions *internal.Conditions
			if output.RawPerformOn != nil { // NOT len(0)
				conditionsCasted := internal.Conditions(output.RawPerformOn)
				conditions = &conditionsCasted
			}

			outputAggregates[oi] = internal.PlanConfig{
				Put:        output.Resource,
				Conditions: conditions,
				Params:     output.Params,
			}
		}

		if len(outputAggregates) > 0 {
			convertedSequence = append(convertedSequence, internal.PlanConfig{Aggregate: &outputAggregates})
		}

		// zero-out old-style config so they're omitted from new payload
		config.Jobs[ji].InputConfigs = nil
		config.Jobs[ji].OutputConfigs = nil
		config.Jobs[ji].TaskConfigPath = ""
		config.Jobs[ji].TaskConfig = nil
		config.Jobs[ji].Privileged = false

		config.Jobs[ji].Plan = convertedSequence
	}

	migratedConfig, err := json.Marshal(config)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE config
		SET config = $1, id = nextval('config_id_seq')
  `, migratedConfig)

	return err
}
