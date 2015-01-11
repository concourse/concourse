package exec

import "os"

//go:generate counterfeiter . CompleteCallback

type CompleteCallback interface {
	Call(error, ArtifactSource)
}

type CallbackFunc func(error, ArtifactSource)

func (f CallbackFunc) Call(err error, source ArtifactSource) {
	f(err, source)
}

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
	step.callback.Call(err, step.ArtifactSource)
	return err
}
