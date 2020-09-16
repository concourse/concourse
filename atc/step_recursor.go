package atc

// StepRecursor is a StepVisitor helper used for traversing a StepConfig and
// calling configured hooks on the "base" step types, i.e. step types that do
// not contain any other steps.
//
// StepRecursor must be updated with any new step type added. Steps which wrap
// other steps must recurse through them, while steps which are "base" steps
// must have a hook added for them, called when they visit the StepRecursor.
type StepRecursor struct {
	// OnTask will be invoked for any *TaskStep present in the StepConfig.
	OnTask func(*TaskStep) error

	// OnGet will be invoked for any *GetStep present in the StepConfig.
	OnGet func(*GetStep) error

	// OnPut will be invoked for any *PutStep present in the StepConfig.
	OnPut func(*PutStep) error

	// OnSetPipeline will be invoked for any *SetPipelineStep present in the StepConfig.
	OnSetPipeline func(*SetPipelineStep) error

	// OnLoadVar will be invoked for any *LoadVarStep present in the StepConfig.
	OnLoadVar func(*LoadVarStep) error
}

// VisitTask calls the OnTask hook if configured.
func (recursor StepRecursor) VisitTask(step *TaskStep) error {
	if recursor.OnTask != nil {
		return recursor.OnTask(step)
	}

	return nil
}

// VisitGet calls the OnGet hook if configured.
func (recursor StepRecursor) VisitGet(step *GetStep) error {
	if recursor.OnGet != nil {
		return recursor.OnGet(step)
	}

	return nil
}

// VisitPut calls the OnPut hook if configured.
func (recursor StepRecursor) VisitPut(step *PutStep) error {
	if recursor.OnPut != nil {
		return recursor.OnPut(step)
	}

	return nil
}

// VisitSetPipeline calls the OnSetPipeline hook if configured.
func (recursor StepRecursor) VisitSetPipeline(step *SetPipelineStep) error {
	if recursor.OnSetPipeline != nil {
		return recursor.OnSetPipeline(step)
	}

	return nil
}

// VisitLoadVar calls the OnLoadVar hook if configured.
func (recursor StepRecursor) VisitLoadVar(step *LoadVarStep) error {
	if recursor.OnLoadVar != nil {
		return recursor.OnLoadVar(step)
	}

	return nil
}

// VisitTry recurses through to the wrapped step.
func (recursor StepRecursor) VisitTry(step *TryStep) error {
	return step.Step.Config.Visit(recursor)
}

// VisitDo recurses through to the wrapped steps.
func (recursor StepRecursor) VisitDo(step *DoStep) error {
	for _, sub := range step.Steps {
		err := sub.Config.Visit(recursor)
		if err != nil {
			return err
		}
	}

	return nil
}

// VisitInParallel recurses through to the wrapped steps.
func (recursor StepRecursor) VisitInParallel(step *InParallelStep) error {
	for _, sub := range step.Config.Steps {
		err := sub.Config.Visit(recursor)
		if err != nil {
			return err
		}
	}

	return nil
}

// VisitAggregate recurses through to the wrapped steps.
func (recursor StepRecursor) VisitAggregate(step *AggregateStep) error {
	for _, sub := range step.Steps {
		err := sub.Config.Visit(recursor)
		if err != nil {
			return err
		}
	}

	return nil
}

// VisitAcross recurses through to the wrapped step.
func (recursor StepRecursor) VisitAcross(step *AcrossStep) error {
	return step.Step.Visit(recursor)
}

// VisitTimeout recurses through to the wrapped step.
func (recursor StepRecursor) VisitTimeout(step *TimeoutStep) error {
	return step.Step.Visit(recursor)
}

// VisitRetry recurses through to the wrapped step.
func (recursor StepRecursor) VisitRetry(step *RetryStep) error {
	return step.Step.Visit(recursor)
}

// VisitOnSuccess recurses through to the wrapped step and hook.
func (recursor StepRecursor) VisitOnSuccess(step *OnSuccessStep) error {
	err := step.Step.Visit(recursor)
	if err != nil {
		return err
	}

	return step.Hook.Config.Visit(recursor)
}

// VisitOnFailure recurses through to the wrapped step and hook.
func (recursor StepRecursor) VisitOnFailure(step *OnFailureStep) error {
	err := step.Step.Visit(recursor)
	if err != nil {
		return err
	}

	return step.Hook.Config.Visit(recursor)
}

// VisitOnAbort recurses through to the wrapped step and hook.
func (recursor StepRecursor) VisitOnAbort(step *OnAbortStep) error {
	err := step.Step.Visit(recursor)
	if err != nil {
		return err
	}

	return step.Hook.Config.Visit(recursor)
}

// VisitOnError recurses through to the wrapped step and hook.
func (recursor StepRecursor) VisitOnError(step *OnErrorStep) error {
	err := step.Step.Visit(recursor)
	if err != nil {
		return err
	}

	return step.Hook.Config.Visit(recursor)
}

// VisitEnsure recurses through to the wrapped step and hook.
func (recursor StepRecursor) VisitEnsure(step *EnsureStep) error {
	err := step.Step.Visit(recursor)
	if err != nil {
		return err
	}

	return step.Hook.Config.Visit(recursor)
}
