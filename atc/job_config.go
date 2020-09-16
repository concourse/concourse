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

	OnSuccess *Step `json:"on_success,omitempty"`
	OnFailure *Step `json:"on_failure,omitempty"`
	OnAbort   *Step `json:"on_abort,omitempty"`
	OnError   *Step `json:"on_error,omitempty"`
	Ensure    *Step `json:"ensure,omitempty"`

	PlanSequence []Step `json:"plan"`
}

type BuildLogRetention struct {
	Builds                 int `json:"builds,omitempty"`
	MinimumSucceededBuilds int `json:"minimum_succeeded_builds,omitempty"`
	Days                   int `json:"days,omitempty"`
}

func (config JobConfig) Step() Step {
	return Step{Config: config.StepConfig()}
}

func (config JobConfig) StepConfig() StepConfig {
	var step StepConfig = &DoStep{
		Steps: config.PlanSequence,
	}

	if config.OnSuccess != nil {
		step = &OnSuccessStep{
			Step: step,
			Hook: *config.OnSuccess,
		}
	}

	if config.OnFailure != nil {
		step = &OnFailureStep{
			Step: step,
			Hook: *config.OnFailure,
		}
	}

	if config.OnAbort != nil {
		step = &OnAbortStep{
			Step: step,
			Hook: *config.OnAbort,
		}
	}

	if config.OnError != nil {
		step = &OnErrorStep{
			Step: step,
			Hook: *config.OnError,
		}
	}

	if config.Ensure != nil {
		step = &EnsureStep{
			Step: step,
			Hook: *config.Ensure,
		}
	}

	return step
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

func (config JobConfig) Inputs() []JobInputParams {
	var inputs []JobInputParams

	_ = config.StepConfig().Visit(StepRecursor{
		OnGet: func(step *GetStep) error {
			inputs = append(inputs, JobInputParams{
				JobInput: JobInput{
					Name:     step.Name,
					Resource: step.ResourceName(),
					Passed:   step.Passed,
					Version:  step.Version,
					Trigger:  step.Trigger,
				},
				Params: step.Params,
				Tags:   step.Tags,
			})

			return nil
		},
	})

	return inputs
}

func (config JobConfig) Outputs() []JobOutput {
	var outputs []JobOutput

	_ = config.StepConfig().Visit(StepRecursor{
		OnPut: func(step *PutStep) error {
			outputs = append(outputs, JobOutput{
				Name:     step.Name,
				Resource: step.ResourceName(),
			})

			return nil
		},
	})

	return outputs
}
