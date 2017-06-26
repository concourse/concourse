package migration_26

import "time"

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
