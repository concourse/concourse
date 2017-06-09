package exec

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

type ConfigFetcher interface {
	FetchConfig(repository *worker.ArtifactRepository) (atc.TaskConfig, error)
}

type FetchConfigResultAction interface {
	Result() atc.TaskConfig
}

type FetchConfigAction struct {
	configFetcher ConfigFetcher
	result        atc.TaskConfig
}

func (action *FetchConfigAction) Run(
	logger lager.Logger,
	repository *worker.ArtifactRepository,

	// TODO: consider passing these as context
	signals <-chan os.Signal,
	ready chan<- struct{},
) error {
	var err error
	action.result, err = action.configFetcher.FetchConfig(repository)
	if err != nil {
		return err
	}

	return nil
}

func (action *FetchConfigAction) Result() atc.TaskConfig {
	return action.result
}

func (action *FetchConfigAction) ExitStatus() ExitStatus {
	return ExitStatus(0)
}
