package atc

type StepRecursor struct {
	OnTask        func(*TaskStep) error
	OnGet         func(*GetStep) error
	OnPut         func(*PutStep) error
	OnSetPipeline func(*SetPipelineStep) error
	OnLoadVar     func(*LoadVarStep) error
}

func (recursor StepRecursor) VisitTask(step *TaskStep) error {
	if recursor.OnTask != nil {
		return recursor.OnTask(step)
	}

	return nil
}

func (recursor StepRecursor) VisitGet(step *GetStep) error {
	if recursor.OnGet != nil {
		return recursor.OnGet(step)
	}

	return nil
}

func (recursor StepRecursor) VisitPut(step *PutStep) error {
	if recursor.OnPut != nil {
		return recursor.OnPut(step)
	}

	return nil
}

func (recursor StepRecursor) VisitSetPipeline(step *SetPipelineStep) error {
	if recursor.OnSetPipeline != nil {
		return recursor.OnSetPipeline(step)
	}

	return nil
}

func (recursor StepRecursor) VisitLoadVar(step *LoadVarStep) error {
	if recursor.OnLoadVar != nil {
		return recursor.OnLoadVar(step)
	}

	return nil
}

func (recursor StepRecursor) VisitTry(step *TryStep) error {
	return step.Step.Config.Visit(recursor)
}

func (recursor StepRecursor) VisitDo(step *DoStep) error {
	for _, sub := range step.Steps {
		err := sub.Config.Visit(recursor)
		if err != nil {
			return err
		}
	}

	return nil
}

func (recursor StepRecursor) VisitInParallel(step *InParallelStep) error {
	for _, sub := range step.Config.Steps {
		err := sub.Config.Visit(recursor)
		if err != nil {
			return err
		}
	}

	return nil
}

func (recursor StepRecursor) VisitAggregate(step *AggregateStep) error {
	for _, sub := range step.Steps {
		err := sub.Config.Visit(recursor)
		if err != nil {
			return err
		}
	}

	return nil
}

func (recursor StepRecursor) VisitTimeout(step *TimeoutStep) error {
	return step.Step.Visit(recursor)
}

func (recursor StepRecursor) VisitRetry(step *RetryStep) error {
	return step.Step.Visit(recursor)
}

func (recursor StepRecursor) VisitOnSuccess(step *OnSuccessStep) error {
	err := step.Step.Visit(recursor)
	if err != nil {
		return err
	}

	return step.Hook.Config.Visit(recursor)
}

func (recursor StepRecursor) VisitOnFailure(step *OnFailureStep) error {
	err := step.Step.Visit(recursor)
	if err != nil {
		return err
	}

	return step.Hook.Config.Visit(recursor)
}

func (recursor StepRecursor) VisitOnAbort(step *OnAbortStep) error {
	err := step.Step.Visit(recursor)
	if err != nil {
		return err
	}

	return step.Hook.Config.Visit(recursor)
}

func (recursor StepRecursor) VisitOnError(step *OnErrorStep) error {
	err := step.Step.Visit(recursor)
	if err != nil {
		return err
	}

	return step.Hook.Config.Visit(recursor)
}

func (recursor StepRecursor) VisitEnsure(step *EnsureStep) error {
	err := step.Step.Visit(recursor)
	if err != nil {
		return err
	}

	return step.Hook.Config.Visit(recursor)
}
