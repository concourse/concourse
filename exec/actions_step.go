package exec

import (
	"os"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . Action

type Action interface {
	Run(lager.Logger, *worker.ArtifactRepository, <-chan os.Signal, chan<- struct{}) error
	ExitStatus() ExitStatus
}

func NewActionsStep(
	logger lager.Logger, // TODO: can we move that to method? need to change all steps though
	actions []Action,
	buildEventsDelegate ActionsBuildEventsDelegate,
) ActionsStep {
	return ActionsStep{
		logger:              logger,
		actions:             actions,
		buildEventsDelegate: buildEventsDelegate,
	}
}

//go:generate counterfeiter . ActionsBuildEventsDelegate

type ActionsBuildEventsDelegate interface {
	ActionCompleted(lager.Logger, Action)
	Failed(lager.Logger, error)
}

// ActionsStep will execute actions in specified order and notify build events
// delegate about different execution events.
type ActionsStep struct {
	actions             []Action
	buildEventsDelegate ActionsBuildEventsDelegate

	logger lager.Logger // TODO: can we move that to method? need to change all steps though

	repository *worker.ArtifactRepository
	succeeded  bool
}

func (s ActionsStep) Using(repo *worker.ArtifactRepository) Step {
	s.repository = repo
	return &s
}

// Run will first call Initializing on build events delegate. Then it will call
// Run on every action. If any action fails it will notify delegate with Failed.
// It will call ActionCompleted after each action run that succeeds.
func (s *ActionsStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	succeeded := true
	for _, action := range s.actions {
		err := action.Run(s.logger, s.repository, signals, ready)
		if err != nil {
			if err == resource.ErrAborted {
				s.logger.Debug("resource-aborted")
				s.buildEventsDelegate.Failed(s.logger, ErrInterrupted)
				return ErrInterrupted
			}

			s.logger.Error("failed-to-run-action", err)
			s.buildEventsDelegate.Failed(s.logger, err)
			return err
		}

		s.buildEventsDelegate.ActionCompleted(s.logger, action)

		if action.ExitStatus() != 0 {
			succeeded = false
		}
	}

	s.succeeded = succeeded

	return nil
}

// Succeeded will return true if all actions exited with exit status 0.
func (s *ActionsStep) Succeeded() bool {
	return s.succeeded
}
