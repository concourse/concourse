package exec

import (
	"context"
	"github.com/concourse/concourse/atc/db"
	"io"

	"code.cloudfoundry.org/lager"
	"go.opentelemetry.io/otel/trace"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/tracing"
)

//counterfeiter:generate . BuildStepDelegateFactory
type BuildStepDelegateFactory interface {
	BuildStepDelegate(state RunState) BuildStepDelegate
}

//counterfeiter:generate . BuildStepDelegate
type BuildStepDelegate interface {
	StartSpan(context.Context, string, tracing.Attrs) (context.Context, trace.Span)

	FetchImage(context.Context, atc.ImageResource, atc.VersionedResourceTypes, bool) (worker.ImageSpec, error)

	Stdout() io.Writer
	Stderr() io.Writer

	Initializing(lager.Logger)
	Starting(lager.Logger)
	Finished(lager.Logger, bool)
	Errored(lager.Logger, string)

	WaitingForWorker(lager.Logger)
	SelectedWorker(lager.Logger, string)

	ConstructAcrossSubsteps([]byte, []atc.AcrossVar, [][]interface{}) ([]atc.VarScopedPlan, error)
	ContainerOwner(planId atc.PlanID) db.ContainerOwner
}

//counterfeiter:generate . SetPipelineStepDelegateFactory
type SetPipelineStepDelegateFactory interface {
	SetPipelineStepDelegate(state RunState) SetPipelineStepDelegate
}

//counterfeiter:generate . SetPipelineStepDelegate
type SetPipelineStepDelegate interface {
	BuildStepDelegate
	SetPipelineChanged(lager.Logger, bool)
	CheckRunSetPipelinePolicy(*atc.Config) error
}
