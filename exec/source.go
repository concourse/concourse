package exec

import "os"

type Source struct {
	Name        SourceName
	StepFactory StepFactory

	step Step
}

func (fact Source) Using(prev Step, repo *SourceRepository) Step {
	return sourceStep{
		Step: fact.StepFactory.Using(prev, repo),

		name: fact.Name,
		repo: repo,
	}
}

type sourceStep struct {
	Step

	name SourceName
	repo *SourceRepository
}

func (step sourceStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := step.Step.Run(signals, ready)
	if err != nil {
		return err
	}

	step.repo.RegisterSource(step.name, step.Step)

	return nil
}
