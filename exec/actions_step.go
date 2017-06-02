package exec

import (
	"os"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/worker"
)

type RootFSSource struct {
	Base   *BaseResourceTypeRootFSSource
	Output *OutputRootFSSource
}

type BaseResourceTypeRootFSSource struct {
	Name string
}

type OutputRootFSSource struct {
	Name string
}

type Action interface {
	Run(lager.Logger, *worker.ArtifactRepository, <-chan os.Signal, chan<- struct{}) error
	Failed(err error)
}

func newActionsStep(
	actions []Action,
	logger lager.Logger, // TODO: can we move that to method? need to change all steps though
) ActionsStep {
	return ActionsStep{
		actions: actions,
		logger:  logger,
	}
}

type ActionsStep struct {
	actions []Action

	logger lager.Logger // TODO: can we move that to method? need to change all steps though

	repository *worker.ArtifactRepository

	succeeded bool
}

func (step ActionsStep) Using(prev Step, repo *worker.ArtifactRepository) Step {
	step.repository = repo

	return &step
}

func (step *ActionsStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	for _, action := range step.actions {
		err := action.Run(step.logger, step.repository, signals, ready)
		if err != nil {
			action.Failed(err)
			return err
		}
	}
	step.succeeded = true

	return nil
}

func (step *ActionsStep) Result(x interface{}) bool {
	switch v := x.(type) {
	case *Success:
		*v = Success(step.succeeded)
		return true

	default:
		return false
	}
}
