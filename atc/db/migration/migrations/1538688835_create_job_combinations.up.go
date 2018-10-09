package migrations

import (
	"database/sql"
	"encoding/json"
)

type planConfig struct {
	Get       string        `yaml:"get,omitempty" json:"get,omitempty" mapstructure:"get"`
	Put       string        `yaml:"put,omitempty" json:"put,omitempty" mapstructure:"put"`
	Resource  string        `yaml:"resource,omitempty" json:"resource,omitempty" mapstructure:"resource"`
	Try       *planConfig   `yaml:"try,omitempty" json:"try,omitempty" mapstructure:"try"`
	Do        *planSequence `yaml:"do,omitempty" json:"do,omitempty" mapstructure:"do"`
	Aggregate *planSequence `yaml:"aggregate,omitempty" json:"aggregate,omitempty" mapstructure:"aggregate"`
	Abort     *planConfig   `yaml:"on_abort,omitempty" json:"on_abort,omitempty" mapstructure:"on_abort"`
	Failure   *planConfig   `yaml:"on_failure,omitempty" json:"on_failure,omitempty" mapstructure:"on_failure"`
	Ensure    *planConfig   `yaml:"ensure,omitempty" json:"ensure,omitempty" mapstructure:"ensure"`
	Success   *planConfig   `yaml:"on_success,omitempty" json:"on_success,omitempty" mapstructure:"on_success"`
}

type planSequence []planConfig

type jobConfig struct {
	Plan    planSequence `yaml:"plan,omitempty" json:"plan,omitempty" mapstructure:"plan"`
	Abort   *planConfig  `yaml:"on_abort,omitempty" json:"on_abort,omitempty" mapstructure:"on_abort"`
	Failure *planConfig  `yaml:"on_failure,omitempty" json:"on_failure,omitempty" mapstructure:"on_failure"`
	Ensure  *planConfig  `yaml:"ensure,omitempty" json:"ensure,omitempty" mapstructure:"ensure"`
	Success *planConfig  `yaml:"on_success,omitempty" json:"on_success,omitempty" mapstructure:"on_success"`
}

type job struct {
	id               int
	config           jobConfig
	buildNumberSeq   int
	inputsDetermined bool
	pipeline_id      int
}

func collectPlans(plan planConfig) planSequence {
	var plans planSequence

	if plan.Abort != nil {
		plans = append(plans, collectPlans(*plan.Abort)...)
	}

	if plan.Success != nil {
		plans = append(plans, collectPlans(*plan.Success)...)
	}

	if plan.Failure != nil {
		plans = append(plans, collectPlans(*plan.Failure)...)
	}

	if plan.Ensure != nil {
		plans = append(plans, collectPlans(*plan.Ensure)...)
	}

	if plan.Try != nil {
		plans = append(plans, collectPlans(*plan.Try)...)
	}

	if plan.Do != nil {
		for _, p := range *plan.Do {
			plans = append(plans, collectPlans(p)...)
		}
	}

	if plan.Aggregate != nil {
		for _, p := range *plan.Aggregate {
			plans = append(plans, collectPlans(p)...)
		}
	}

	return append(plans, plan)
}

func resourceSpaceCombinations(resourceSpaces map[string][]string) []map[string]string {
	var resource, space string
	var spaces []string

	combinations := []map[string]string{}

	if len(resourceSpaces) == 0 {
		return []map[string]string{map[string]string{}}
	}

	if len(resourceSpaces) == 1 {
		for resource, spaces = range resourceSpaces {
			for _, space = range spaces {
				combinations = append(combinations, map[string]string{resource: space})
			}
		}
		return combinations
	}

	for resource, spaces = range resourceSpaces {
		break
	}
	delete(resourceSpaces, resource)

	for _, combination := range resourceSpaceCombinations(resourceSpaces) {
		for _, space = range spaces {
			copy := map[string]string{}
			for k, v := range combination {
				copy[k] = v
			}

			copy[resource] = space
			combinations = append(combinations, copy)
		}
	}

	return combinations
}

func (config jobConfig) plans() planSequence {
	plan := collectPlans(planConfig{
		Do:      &config.Plan,
		Abort:   config.Abort,
		Ensure:  config.Ensure,
		Failure: config.Failure,
		Success: config.Success,
	})

	return plan
}

func (config jobConfig) spaces() map[string][]string {
	spaces := map[string][]string{}

	for _, plan := range config.plans() {
		resource := plan.Resource

		if resource == "" {
			resource = plan.Get
		}
		if resource == "" {
			resource = plan.Put
		}
		if resource != "" {
			spaces[resource] = []string{"default"}
		}
	}

	return spaces
}

func (self *migrations) Up_1515427942() error {
	tx, err := self.DB.Begin()
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.Exec(`CREATE TABLE job_combinations (
    id SERIAL PRIMARY KEY,
    job_id int REFERENCES jobs (id) ON DELETE CASCADE,
    combination jsonb NOT NULL,
    build_number_seq integer DEFAULT 0 NOT NULL,
    inputs_determined boolean DEFAULT false,
    latest_completed_build_id integer REFERENCES builds (id) ON DELETE SET NULL,
    next_build_id integer REFERENCES builds (id) ON DELETE SET NULL,
    transition_build_id integer REFERENCES builds (id) ON DELETE SET NULL
  );`)
	if err != nil {
		return err
	}

	rows, err := tx.Query("SELECT id, config, build_number_seq, inputs_determined, nonce, pipeline_id FROM jobs WHERE active='true'")
	if err != nil {
		return err
	}

	var configBlob []byte
	var nonce sql.NullString
	var noncense *string

	jobs := []job{}
	for rows.Next() {
		job := job{}

		err = rows.Scan(&job.id, &configBlob, &job.buildNumberSeq, &job.inputsDetermined, &nonce, &job.pipeline_id)
		if err != nil {
			return err
		}

		if nonce.Valid {
			noncense = &nonce.String
		}

		decryptedConfig, err := self.Strategy.Decrypt(string(configBlob), noncense)
		if err != nil {
			return err
		}

		var config jobConfig
		err = json.Unmarshal(decryptedConfig, &config)
		if err != nil {
			return err
		}

		job.config = config

		jobs = append(jobs, job)
	}

	for _, job := range jobs {
		resourceSpaces := job.config.spaces()
		combinations := resourceSpaceCombinations(resourceSpaces)

		for _, combination := range combinations {
			marshaled, err := json.Marshal(combination)
			if err != nil {
				return err
			}

			_, err = tx.Exec(`
				INSERT INTO job_combinations(id, job_id, combination, build_number_seq, inputs_determined)
				VALUES ($1, $2, $3, $4, $5)
			`, job.id, job.id, marshaled, job.buildNumberSeq, job.inputsDetermined)
			if err != nil {
				return err
			}

			// var resourceSpaceID int

			// for resource, space := range combination {
			// 	err = tx.QueryRow(`
			// 		SELECT rs.id FROM resources r, resource_spaces rs
			// 		WHERE r.id = rs.resource_id
			// 		AND r.pipeline_id=$1
			// 		AND r.name=$2
			// 		AND rs.name=$3
			// 	`, job.pipeline_id, resource, space).Scan(&resourceSpaceID)
			// 	if err != nil {
			// 		return err
			// 	}

			// 	_, err = tx.Exec(`
			// 		INSERT INTO job_combination_spaces(job_combination_id, resource_space_id)
			// 		VALUES ($1, $2)
			// 	`, job.id, resourceSpaceID)
			// 	if err != nil {
			// 		return err
			// 	}
			// }
		}
	}

	return tx.Commit()
}
