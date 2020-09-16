package exec

import (
	"io"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/vars"
)

//go:generate counterfeiter . BuildStepDelegate

type BuildStepDelegate interface {
	ImageVersionDetermined(db.UsedResourceCache) error
	RedactImageSource(source atc.Source) (atc.Source, error)

	Stdout() io.Writer
	Stderr() io.Writer

	Variables() *vars.BuildVariables

	Initializing(lager.Logger)
	Starting(lager.Logger)
	Finished(lager.Logger, bool)
	SelectedWorker(lager.Logger, string)
	Errored(lager.Logger, string)
}

//go:generate counterfeiter . SetPipelineStepDelegate

type SetPipelineStepDelegate interface {
	BuildStepDelegate
	SetPipelineChanged(lager.Logger, bool)
}
