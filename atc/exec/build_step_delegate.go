package exec

import (
	"context"
	"io"

	"code.cloudfoundry.org/lager"
	"go.opentelemetry.io/otel/trace"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
)

//counterfeiter:generate . BuildStepDelegateFactory
type BuildStepDelegateFactory interface {
	BuildStepDelegate(state RunState) BuildStepDelegate
}

//counterfeiter:generate . BuildStepDelegate
type BuildStepDelegate interface {
	StartSpan(context.Context, string, tracing.Attrs) (context.Context, trace.Span)

	Variables(context.Context) vars.Variables
	VarSources() atc.VarSourceConfigs
	FetchImage(context.Context, atc.Plan, *atc.Plan, bool) (worker.ImageSpec, db.ResourceCache, error)

	Stdout() io.Writer
	Stderr() io.Writer

	Initializing(lager.Logger)
	Starting(lager.Logger)
	Finished(lager.Logger, bool)
	Errored(lager.Logger, string)

	WaitingForWorker(lager.Logger)
	SelectedWorker(lager.Logger, string)
}

//counterfeiter:generate . SetPipelineStepDelegateFactory
type SetPipelineStepDelegateFactory interface {
	SetPipelineStepDelegate(state RunState) SetPipelineStepDelegate
}

//counterfeiter:generate . SetPipelineStepDelegate
type SetPipelineStepDelegate interface {
	BuildStepDelegate
	SetPipelineChanged(lager.Logger, bool)
}
