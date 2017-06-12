package exec

import (
	"os"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/resource"
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
	ExitStatus() ExitStatus
}

func newActionsStep(
	logger lager.Logger, // TODO: can we move that to method? need to change all steps though
	actions []Action,
	buildEventsDelegate BuildEventsDelegate,
) ActionsStep {
	return ActionsStep{
		logger:              logger,
		actions:             actions,
		buildEventsDelegate: buildEventsDelegate,
	}
}

//go:generate counterfeiter . BuildEventsDelegate

type BuildEventsDelegate interface {
	Initializing(lager.Logger)
	ActionCompleted(lager.Logger, Action)
	Failed(lager.Logger, error)
}

type ActionsStep struct {
	actions             []Action
	buildEventsDelegate BuildEventsDelegate

	logger lager.Logger // TODO: can we move that to method? need to change all steps though

	repository *worker.ArtifactRepository
	succeeded  bool
}

func (s ActionsStep) Using(repo *worker.ArtifactRepository) Step {
	s.repository = repo
	return &s
}

func (s *ActionsStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	s.buildEventsDelegate.Initializing(s.logger)

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

func (s *ActionsStep) Succeeded() bool {
	return s.succeeded
}
