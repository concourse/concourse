package exec

import (
	"errors"
	"time"
)

const successfulStepTTL = 5 * time.Minute
const failedStepTTL = 1 * time.Hour

var ErrInterrupted = errors.New("interrupted")

//go:generate counterfeiter . StepFactory

type StepFactory interface {
	Using(Step, *SourceRepository) Step
}
