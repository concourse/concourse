package migrations

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

type V5JobConfigs []V5JobConfig

type V5JobConfig struct {
	Name                 string   `json:"name"`
	Public               bool     `json:"public,omitempty"`
	DisableManualTrigger bool     `json:"disable_manual_trigger,omitempty"`
	Serial               bool     `json:"serial,omitempty"`
	SerialGroups         []string `json:"serial_groups,omitempty"`
	RawMaxInFlight       int      `json:"max_in_flight,omitempty"`

	Abort   *V5PlanConfig `json:"on_abort,omitempty"`
	Error   *V5PlanConfig `json:"on_error,omitempty"`
	Failure *V5PlanConfig `json:"on_failure,omitempty"`
	Ensure  *V5PlanConfig `json:"ensure,omitempty"`
	Success *V5PlanConfig `json:"on_success,omitempty"`

	Plan V5PlanSequence `json:"plan"`
}

type V5PlanSequence []V5PlanConfig

func (config V5JobConfig) Plans() []V5PlanConfig {
	plan := v5collectPlans(V5PlanConfig{
		Do:      &config.Plan,
		Abort:   config.Abort,
		Error:   config.Error,
		Ensure:  config.Ensure,
		Failure: config.Failure,
		Success: config.Success,
	})

	return plan
}

func (config V5JobConfig) MaxInFlight() int {
	if config.Serial || len(config.SerialGroups) > 0 {
		return 1
	}

	if config.RawMaxInFlight != 0 {
		return config.RawMaxInFlight
	}

	return 0
}

func v5collectPlans(plan V5PlanConfig) []V5PlanConfig {
	var plans []V5PlanConfig

	if plan.Abort != nil {
		plans = append(plans, v5collectPlans(*plan.Abort)...)
	}

	if plan.Error != nil {
		plans = append(plans, v5collectPlans(*plan.Error)...)
	}

	if plan.Success != nil {
		plans = append(plans, v5collectPlans(*plan.Success)...)
	}

	if plan.Failure != nil {
		plans = append(plans, v5collectPlans(*plan.Failure)...)
	}

	if plan.Ensure != nil {
		plans = append(plans, v5collectPlans(*plan.Ensure)...)
	}

	if plan.Try != nil {
		plans = append(plans, v5collectPlans(*plan.Try)...)
	}

	if plan.Do != nil {
		for _, p := range *plan.Do {
			plans = append(plans, v5collectPlans(p)...)
		}
	}

	if plan.Aggregate != nil {
		for _, p := range *plan.Aggregate {
			plans = append(plans, v5collectPlans(p)...)
		}
	}

	if plan.InParallel != nil {
		for _, p := range plan.InParallel.Steps {
			plans = append(plans, v5collectPlans(p)...)
		}
	}

	return append(plans, plan)
}

type V5InParallelConfig struct {
	Steps    V5PlanSequence `json:"steps,omitempty"`
	Limit    int            `json:"limit,omitempty"`
	FailFast bool           `json:"fail_fast,omitempty"`
}

type V5VersionConfig struct {
	Every  bool
	Latest bool
	Pinned Version
}

const V5VersionLatest = "latest"
const V5VersionEvery = "every"

func (c *V5VersionConfig) UnmarshalJSON(version []byte) error {
	var data interface{}

	err := json.Unmarshal(version, &data)
	if err != nil {
		return err
	}

	switch actual := data.(type) {
	case string:
		c.Every = actual == "every"
		c.Latest = actual == "latest"
	case map[string]interface{}:
		version := Version{}

		for k, v := range actual {
			if s, ok := v.(string); ok {
				version[k] = s
				continue
			}

			return fmt.Errorf("the value %v of %s is not a string", v, k)
		}

		c.Pinned = version
	default:
		return errors.New("unknown type for version")
	}

	return nil
}

func (c *V5VersionConfig) MarshalJSON() ([]byte, error) {
	if c.Latest {
		return json.Marshal(V5VersionLatest)
	}

	if c.Every {
		return json.Marshal(V5VersionEvery)
	}

	if c.Pinned != nil {
		return json.Marshal(c.Pinned)
	}

	return json.Marshal("")
}

type V5PlanConfig struct {
	Do         *V5PlanSequence     `json:"do,omitempty"`
	Aggregate  *V5PlanSequence     `json:"aggregate,omitempty"`
	InParallel *V5InParallelConfig `json:"in_parallel,omitempty"`
	Get        string              `json:"get,omitempty"`
	Passed     []string            `json:"passed,omitempty"`
	Trigger    bool                `json:"trigger,omitempty"`
	Put        string              `json:"put,omitempty"`
	Resource   string              `json:"resource,omitempty"`
	Abort      *V5PlanConfig       `json:"on_abort,omitempty"`
	Error      *V5PlanConfig       `json:"on_error,omitempty"`
	Failure    *V5PlanConfig       `json:"on_failure,omitempty"`
	Ensure     *V5PlanConfig       `json:"ensure,omitempty"`
	Success    *V5PlanConfig       `json:"on_success,omitempty"`
	Try        *V5PlanConfig       `json:"try,omitempty"`
	Version    *V5VersionConfig    `json:"version,omitempty"`
}

func (self *migrations) Up_1574452410() error {
	tx, err := self.DB.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	rows, err := tx.Query("SELECT pipeline_id, config, nonce FROM jobs WHERE active = true")
	if err != nil {
		return err
	}

	pipelineJobConfigs := make(map[int]V5JobConfigs)
	for rows.Next() {
		var configBlob []byte
		var nonce sql.NullString
		var pipelineID int

		err = rows.Scan(&pipelineID, &configBlob, &nonce)
		if err != nil {
			return err
		}

		var noncense *string
		if nonce.Valid {
			noncense = &nonce.String
		}

		decrypted, err := self.Strategy.Decrypt(string(configBlob), noncense)
		if err != nil {
			return err
		}

		var config V5JobConfig
		err = json.Unmarshal(decrypted, &config)
		if err != nil {
			return err
		}

		pipelineJobConfigs[pipelineID] = append(pipelineJobConfigs[pipelineID], config)
	}

	for pipelineID, jobConfigs := range pipelineJobConfigs {
		resourceNameToID := make(map[string]int)
		jobNameToID := make(map[string]int)

		rows, err := tx.Query("SELECT id, name FROM resources WHERE pipeline_id = $1", pipelineID)
		if err != nil {
			return err
		}

		for rows.Next() {
			var id int
			var name string

			err = rows.Scan(&id, &name)
			if err != nil {
				return err
			}

			resourceNameToID[name] = id
		}

		rows, err = tx.Query("SELECT id, name FROM jobs WHERE pipeline_id = $1", pipelineID)
		if err != nil {
			return err
		}

		for rows.Next() {
			var id int
			var name string

			err = rows.Scan(&id, &name)
			if err != nil {
				return err
			}

			jobNameToID[name] = id
		}

		_, err = tx.Exec(`
			DELETE FROM jobs_serial_groups
			WHERE job_id in (
				SELECT j.id
				FROM jobs j
				WHERE j.pipeline_id = $1
			)`, pipelineID)
		if err != nil {
			return err
		}

		for _, jobConfig := range jobConfigs {
			if len(jobConfig.SerialGroups) != 0 {
				for _, sg := range jobConfig.SerialGroups {
					err = registerSerialGroup(tx, sg, jobNameToID[jobConfig.Name])
					if err != nil {
						return err
					}
				}
			} else {
				if jobConfig.Serial || jobConfig.RawMaxInFlight > 0 {
					err = registerSerialGroup(tx, jobConfig.Name, jobNameToID[jobConfig.Name])
					if err != nil {
						return err
					}
				}
			}

			for _, plan := range jobConfig.Plans() {
				if plan.Get != "" {
					err = insertJobInput(tx, plan, jobConfig.Name, resourceNameToID, jobNameToID)
					if err != nil {
						return err
					}
				} else if plan.Put != "" {
					err = insertJobOutput(tx, plan, jobConfig.Name, resourceNameToID, jobNameToID)
					if err != nil {
						return err
					}
				}
			}

			err = updateJob(tx, jobConfig, jobNameToID)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func insertJobInput(tx *sql.Tx, plan V5PlanConfig, jobName string, resourceNameToID map[string]int, jobNameToID map[string]int) error {
	if len(plan.Passed) != 0 {
		for _, passedJob := range plan.Passed {
			var resourceID int
			if plan.Resource != "" {
				resourceID = resourceNameToID[plan.Resource]
			} else {
				resourceID = resourceNameToID[plan.Get]
			}

			var version sql.NullString
			if plan.Version != nil {
				versionJSON, err := plan.Version.MarshalJSON()
				if err != nil {
					return err
				}

				version = sql.NullString{Valid: true, String: string(versionJSON)}
			}

			_, err := tx.Exec("INSERT INTO job_inputs (name, job_id, resource_id, passed_job_id, trigger, version) VALUES ($1, $2, $3, $4, $5, $6)", plan.Get, jobNameToID[jobName], resourceID, jobNameToID[passedJob], plan.Trigger, version)
			if err != nil {
				return err
			}
		}
	} else {
		var resourceID int
		if plan.Resource != "" {
			resourceID = resourceNameToID[plan.Resource]
		} else {
			resourceID = resourceNameToID[plan.Get]
		}

		var version sql.NullString
		if plan.Version != nil {
			versionJSON, err := plan.Version.MarshalJSON()
			if err != nil {
				return err
			}

			version = sql.NullString{Valid: true, String: string(versionJSON)}
		}

		_, err := tx.Exec("INSERT INTO job_inputs (name, job_id, resource_id, trigger, version) VALUES ($1, $2, $3, $4, $5)", plan.Get, jobNameToID[jobName], resourceID, plan.Trigger, version)
		if err != nil {
			return err
		}
	}

	return nil
}

func insertJobOutput(tx *sql.Tx, plan V5PlanConfig, jobName string, resourceNameToID map[string]int, jobNameToID map[string]int) error {
	var resourceID int
	if plan.Resource != "" {
		resourceID = resourceNameToID[plan.Resource]
	} else {
		resourceID = resourceNameToID[plan.Put]
	}

	_, err := tx.Exec("INSERT INTO job_outputs (name, job_id, resource_id) VALUES ($1, $2, $3)", plan.Put, jobNameToID[jobName], resourceID)
	if err != nil {
		return err
	}

	return nil
}

func updateJob(tx *sql.Tx, jobConfig V5JobConfig, jobNameToID map[string]int) error {
	_, err := tx.Exec("UPDATE jobs SET public = $1, max_in_flight = $2, disable_manual_trigger = $3 WHERE id = $4", jobConfig.Public, jobConfig.MaxInFlight(), jobConfig.DisableManualTrigger, jobNameToID[jobConfig.Name])
	if err != nil {
		return err
	}

	return nil
}

func registerSerialGroup(tx *sql.Tx, serialGroup string, jobID int) error {
	_, err := tx.Exec("INSERT INTO jobs_serial_groups (serial_group, job_id) VALUES ($1, $2)", serialGroup, jobID)
	return err
}
