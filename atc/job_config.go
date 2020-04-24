package atc

type JobConfig struct {
	Name    string `json:"name"`
	OldName string `json:"old_name,omitempty"`
	Public  bool   `json:"public,omitempty"`

	DisableManualTrigger bool     `json:"disable_manual_trigger,omitempty"`
	Serial               bool     `json:"serial,omitempty"`
	Interruptible        bool     `json:"interruptible,omitempty"`
	SerialGroups         []string `json:"serial_groups,omitempty"`
	RawMaxInFlight       int      `json:"max_in_flight,omitempty"`
	BuildLogsToRetain    int      `json:"build_logs_to_retain,omitempty"`

	BuildLogRetention *BuildLogRetention `json:"build_log_retention,omitempty"`

	Abort   *PlanConfig `json:"on_abort,omitempty"`
	Error   *PlanConfig `json:"on_error,omitempty"`
	Failure *PlanConfig `json:"on_failure,omitempty"`
	Ensure  *PlanConfig `json:"ensure,omitempty"`
	Success *PlanConfig `json:"on_success,omitempty"`

	PlanSequence PlanSequence `json:"plan"`
}

type BuildLogRetention struct {
	Builds                 int `json:"builds,omitempty"`
	MinimumSucceededBuilds int `json:"minimum_succeeded_builds,omitempty"`
	Days                   int `json:"days,omitempty"`
}

func (config JobConfig) PlanConfig() PlanConfig {
	return PlanConfig{
		Do:      &config.PlanSequence,
		Abort:   config.Abort,
		Error:   config.Error,
		Failure: config.Failure,
		Ensure:  config.Ensure,
		Success: config.Success,
	}
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

func (config JobConfig) InputPlans() []PlanConfig {
	var inputs []PlanConfig

	config.PlanConfig().Each(func(plan PlanConfig) {
		if plan.Get != "" {
			inputs = append(inputs, plan)
		}
	})

	return inputs
}

func (config JobConfig) OutputPlans() []PlanConfig {
	var outputs []PlanConfig

	config.PlanConfig().Each(func(plan PlanConfig) {
		if plan.Put != "" {
			outputs = append(outputs, plan)
		}
	})

	return outputs
}

func (config JobConfig) Inputs() []JobInputParams {
	var inputs []JobInputParams

	config.PlanConfig().Each(func(plan PlanConfig) {
		if plan.Get != "" {
			get := plan.Get

			resource := get
			if plan.Resource != "" {
				resource = plan.Resource
			}

			inputs = append(inputs, JobInputParams{
				JobInput: JobInput{
					Name:     get,
					Resource: resource,
					Passed:   plan.Passed,
					Version:  plan.Version,
					Trigger:  plan.Trigger,
				},
				Params: plan.Params,
				Tags:   plan.Tags,
			})
		}
	})

	return inputs
}

func (config JobConfig) Outputs() []JobOutput {
	var outputs []JobOutput

	config.PlanConfig().Each(func(plan PlanConfig) {
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
	})

	return outputs
}
