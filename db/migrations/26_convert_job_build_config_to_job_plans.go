package migrations

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/concourse/atc/dbng/migration"
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

	var config Config
	err = json.Unmarshal(configPayload, &config)
	if err != nil {
		return err
	}

	for ji, job := range config.Jobs {
		if len(job.Plan) > 0 { // skip jobs already converted to plans
			continue
		}

		convertedSequence := PlanSequence{}

		inputAggregates := make(PlanSequence, len(job.InputConfigs))
		for ii, input := range job.InputConfigs {
			name := input.RawName
			resource := input.Resource
			if name == "" {
				name = input.Resource
				resource = ""
			}

			inputAggregates[ii] = PlanConfig{
				Get:        name,
				Resource:   resource,
				RawTrigger: input.RawTrigger,
				Passed:     input.Passed,
				Params:     input.Params,
			}
		}

		if len(inputAggregates) > 0 {
			convertedSequence = append(convertedSequence, PlanConfig{Aggregate: &inputAggregates})
		}

		if job.TaskConfig != nil || job.TaskConfigPath != "" {
			convertedSequence = append(convertedSequence, PlanConfig{
				Task:           "build", // default name
				TaskConfigPath: job.TaskConfigPath,
				TaskConfig:     job.TaskConfig,
				Privileged:     job.Privileged,
			})
		}

		outputAggregates := make(PlanSequence, len(job.OutputConfigs))
		for oi, output := range job.OutputConfigs {
			var conditions *Conditions
			if output.RawPerformOn != nil { // NOT len(0)
				conditionsCasted := Conditions(output.RawPerformOn)
				conditions = &conditionsCasted
			}

			outputAggregates[oi] = PlanConfig{
				Put:        output.Resource,
				Conditions: conditions,
				Params:     output.Params,
			}
		}

		if len(outputAggregates) > 0 {
			convertedSequence = append(convertedSequence, PlanConfig{Aggregate: &outputAggregates})
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
	if err != nil {
		return err
	}

	return nil
}

type Source map[string]interface{}
type Params map[string]interface{}
type Version map[string]interface{}

type Config struct {
	Groups    GroupConfigs    `json:"groups,omitempty"`
	Resources ResourceConfigs `json:"resources,omitempty"`
	Jobs      JobConfigs      `json:"jobs,omitempty"`
}

type GroupConfig struct {
	Name      string   `json:"name"`
	Jobs      []string `json:"jobs,omitempty"`
	Resources []string `json:"resources,omitempty"`
}

type GroupConfigs []GroupConfig

type ResourceConfig struct {
	Name string `json:"name"`

	Type   string `json:"type"`
	Source Source `json:"source"`
}

type JobConfig struct {
	Name   string `json:"name"`
	Public bool   `json:"public,omitempty"`
	Serial bool   `json:"serial,omitempty"`

	Privileged     bool        `json:"privileged,omitempty"`
	TaskConfigPath string      `json:"build,omitempty"`
	TaskConfig     *TaskConfig `json:"config,omitempty"`

	InputConfigs  []JobInputConfig  `json:"inputs,omitempty"`
	OutputConfigs []JobOutputConfig `json:"outputs,omitempty"`

	Plan PlanSequence `json:"plan,omitempty"`
}

type PlanSequence []PlanConfig

type PlanConfig struct {
	Conditions *Conditions `json:"conditions,omitempty"`

	RawName string `json:"name,omitempty"`

	Do *PlanSequence `json:"do,omitempty"`

	Aggregate *PlanSequence `json:"aggregate,omitempty"`

	Get        string   `json:"get,omitempty"`
	Passed     []string `json:"passed,omitempty"`
	RawTrigger *bool    `json:"trigger,omitempty"`

	Put string `json:"put,omitempty"`

	Resource string `json:"resource,omitempty"`

	Task           string      `json:"task,omitempty"`
	Privileged     bool        `json:"privileged,omitempty"`
	TaskConfigPath string      `json:"file,omitempty"`
	TaskConfig     *TaskConfig `json:"config,omitempty"`

	Params Params `json:"params,omitempty"`
}

type JobInputConfig struct {
	RawName    string   `json:"name,omitempty"`
	Resource   string   `json:"resource"`
	Params     Params   `json:"params,omitempty"`
	Passed     []string `json:"passed,omitempty"`
	RawTrigger *bool    `json:"trigger"`
}

type JobOutputConfig struct {
	Resource string `json:"resource"`
	Params   Params `json:"params,omitempty"`

	RawPerformOn []Condition `json:"perform_on,omitempty"`
}

type Conditions []Condition

type Condition string

const (
	ConditionSuccess Condition = "success"
	ConditionFailure Condition = "failure"
)

type Duration time.Duration

type ResourceConfigs []ResourceConfig

type JobConfigs []JobConfig

type TaskConfig struct {
	Platform string `json:"platform,omitempty"`

	Tags []string `json:"tags,omitempty"`

	Image string `json:"image,omitempty"`

	Params map[string]string `json:"params,omitempty"`

	Run *TaskRunConfig `json:"run,omitempty"`

	Inputs []TaskInputConfig `json:"inputs,omitempty"`
}

type TaskRunConfig struct {
	Path string   `json:"path,omitempty"`
	Args []string `json:"args,omitempty"`
}

type TaskInputConfig struct {
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
}
