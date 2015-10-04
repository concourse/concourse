package exec

import "time"

const successfulStepTTL = 5 * time.Minute
const failedStepTTL = 1 * time.Hour

//go:generate counterfeiter . StepFactory

type StepFactory interface {
	Using(Step, *SourceRepository) Step
}
