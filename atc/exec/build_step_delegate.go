package exec

import (
	"context"
	"io"
	"time"

	"code.cloudfoundry.org/lager"
	"go.opentelemetry.io/otel/trace"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/tracing"
)

//counterfeiter:generate . BuildStepDelegateFactory
type BuildStepDelegateFactory interface {
	BuildStepDelegate(state RunState) BuildStepDelegate
}

//counterfeiter:generate . BuildStepDelegate
type BuildStepDelegate interface {
	StartSpan(context.Context, string, tracing.Attrs) (context.Context, trace.Span)

	FetchImage(context.Context, atc.Plan, *atc.Plan, bool) (runtime.ImageSpec, db.ResourceCache, error)

	Stdout() io.Writer
	Stderr() io.Writer

	Initializing(lager.Logger)
	Starting(lager.Logger)
	Finished(lager.Logger, bool)
	Errored(lager.Logger, string)

	BeforeSelectWorker(lager.Logger) error
	WaitingForWorker(lager.Logger)
	SelectedWorker(lager.Logger, string)
	StreamingVolume(lager.Logger, string, string, string)
	WaitingForStreamedVolume(lager.Logger, string, string)
	BuildStartTime() time.Time

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
