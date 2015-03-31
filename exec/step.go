package exec

//go:generate counterfeiter . StepFactory

type StepFactory interface {
	Using(Step, *SourceRepository) Step
}
