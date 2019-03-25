package atc

type JobConfig struct {
	Name   string `yaml:"name" json:"name" mapstructure:"name"`
	Public bool   `yaml:"public,omitempty" json:"public,omitempty" mapstructure:"public"`

	DisableManualTrigger bool     `yaml:"disable_manual_trigger,omitempty" json:"disable_manual_trigger,omitempty" mapstructure:"disable_manual_trigger"`
	Serial               bool     `yaml:"serial,omitempty" json:"serial,omitempty" mapstructure:"serial"`
	Interruptible        bool     `yaml:"interruptible,omitempty" json:"interruptible,omitempty" mapstructure:"interruptible"`
	SerialGroups         []string `yaml:"serial_groups,omitempty" json:"serial_groups,omitempty" mapstructure:"serial_groups"`
	RawMaxInFlight       int      `yaml:"max_in_flight,omitempty" json:"max_in_flight,omitempty" mapstructure:"max_in_flight"`
	BuildLogsToRetain    int      `yaml:"build_logs_to_retain,omitempty" json:"build_logs_to_retain,omitempty" mapstructure:"build_logs_to_retain"`

	Plan PlanSequence `yaml:"plan,omitempty" json:"plan,omitempty" mapstructure:"plan"`

	Abort   *PlanConfig `yaml:"on_abort,omitempty" json:"on_abort,omitempty" mapstructure:"on_abort"`
	Error   *PlanConfig `yaml:"on_error,omitempty" json:"on_error,omitempty" mapstructure:"on_error"`
	Failure *PlanConfig `yaml:"on_failure,omitempty" json:"on_failure,omitempty" mapstructure:"on_failure"`
	Ensure  *PlanConfig `yaml:"ensure,omitempty" json:"ensure,omitempty" mapstructure:"ensure"`
	Success *PlanConfig `yaml:"on_success,omitempty" json:"on_success,omitempty" mapstructure:"on_success"`
}

func (config JobConfig) Hooks() Hooks {
	return Hooks{Abort: config.Abort, Error: config.Error, Failure: config.Failure, Ensure: config.Ensure, Success: config.Success}
}

func (config JobConfig) MaxInFlight() int {
	if config.Serial || len(config.SerialGroups) > 0 {
		return 1
	}

	if config.RawMaxInFlight != 0 {
		return config.RawMaxInFlight
	}

	return 0
}

func (config JobConfig) GetSerialGroups() []string {
	if len(config.SerialGroups) > 0 {
		return config.SerialGroups
	}

	if config.Serial || config.RawMaxInFlight > 0 {
		return []string{config.Name}
	}

	return []string{}
}

func (config JobConfig) Plans() []PlanConfig {
	plan := collectPlans(PlanConfig{
		Do:      &config.Plan,
		Abort:   config.Abort,
		Error:   config.Error,
		Ensure:  config.Ensure,
		Failure: config.Failure,
		Success: config.Success,
	})

	return plan
}

func collectPlans(plan PlanConfig) []PlanConfig {
	var plans []PlanConfig

	if plan.Abort != nil {
		plans = append(plans, collectPlans(*plan.Abort)...)
	}

	if plan.Error != nil {
		plans = append(plans, collectPlans(*plan.Error)...)
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

func (config JobConfig) InputPlans() []PlanConfig {
	var inputs []PlanConfig

	for _, plan := range config.Plans() {
		if plan.Get != "" {
			inputs = append(inputs, plan)
		}
	}

	return inputs
}

func (config JobConfig) OutputPlans() []PlanConfig {
	var outputs []PlanConfig

	for _, plan := range config.Plans() {
		if plan.Put != "" {
			outputs = append(outputs, plan)
		}
	}

	return outputs
}

func (config JobConfig) Inputs() []JobInput {
	var inputs []JobInput

	for _, plan := range config.Plans() {
		if plan.Get != "" {
			get := plan.Get

			resource := get
			if plan.Resource != "" {
				resource = plan.Resource
			}

			inputs = append(inputs, JobInput{
				Name:     get,
				Resource: resource,
				Passed:   plan.Passed,
				Version:  plan.Version,
				Trigger:  plan.Trigger,
				Params:   plan.Params,
				Tags:     plan.Tags,
			})
		}
	}

	return inputs
}

func (config JobConfig) Outputs() []JobOutput {
	var outputs []JobOutput

	for _, plan := range config.Plans() {
		if plan.Put != "" {
			put := plan.Put

			resource := put
			if plan.Resource != "" {
				resource = plan.Resource
			}

			outputs = append(outputs, JobOutput{
				Name:     put,
				Resource: resource,
			})
		}
	}

	return outputs
}
