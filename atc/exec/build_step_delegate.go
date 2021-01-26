package exec

import (
	"context"
	"io"

	"code.cloudfoundry.org/lager"
	"go.opentelemetry.io/otel/api/trace"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
)

//go:generate counterfeiter . BuildStepDelegateFactory

type BuildStepDelegateFactory interface {
	BuildStepDelegate(state RunState) BuildStepDelegate
}

//go:generate counterfeiter . BuildStepDelegate

type BuildStepDelegate interface {
	StartSpan(context.Context, string, tracing.Attrs) (context.Context, trace.Span)

	Variables(context.Context) vars.Variables
	VarSources() atc.VarSourceConfigs
	FetchImage(context.Context, atc.ImageResource, atc.VersionedResourceTypes, bool) (worker.ImageSpec, error)

	Stdout() io.Writer
	Stderr() io.Writer

	Initializing(lager.Logger)
	Starting(lager.Logger)
	Finished(lager.Logger, bool)
	SelectedWorker(lager.Logger, string)
	Errored(lager.Logger, string)
}

//go:generate counterfeiter . SetPipelineStepDelegateFactory

type SetPipelineStepDelegateFactory interface {
	SetPipelineStepDelegate(state RunState) SetPipelineStepDelegate
}

//go:generate counterfeiter . SetPipelineStepDelegate

type SetPipelineStepDelegate interface {
	BuildStepDelegate
	SetPipelineChanged(lager.Logger, bool)
}
