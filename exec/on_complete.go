package exec

import "os"

type CompleteCallback func(error, ArtifactSource)

func OnComplete(step Step, callback CompleteCallback) Step {
	return onComplete{step: step, callback: callback}
}

type onComplete struct {
	step     Step
	callback CompleteCallback

	ArtifactSource
}

func (step onComplete) Using(source ArtifactSource) ArtifactSource {
	step.ArtifactSource = step.step.Using(source)
	return &step
}

func (step *onComplete) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := step.ArtifactSource.Run(signals, ready)
	step.callback(err, step.ArtifactSource)
	return err
}
