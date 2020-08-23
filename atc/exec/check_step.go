package exec

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/tracing"
)

type CheckStep struct {
	planID                atc.PlanID
	plan                  atc.CheckPlan
	metadata              StepMetadata
	containerMetadata     db.ContainerMetadata
	resourceFactory       resource.ResourceFactory
	resourceConfigFactory db.ResourceConfigFactory
	strategy              worker.ContainerPlacementStrategy
	pool                  worker.Pool
	delegate              CheckDelegate
	succeeded             bool
	workerClient          worker.Client
}

//go:generate counterfeiter . CheckDelegate

type CheckDelegate interface {
	BuildStepDelegate

	SaveVersions(db.SpanContext, db.ResourceConfig, []atc.Version) error
}

func NewCheckStep(
	planID atc.PlanID,
	plan atc.CheckPlan,
	metadata StepMetadata,
	resourceFactory resource.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	containerMetadata db.ContainerMetadata,
	strategy worker.ContainerPlacementStrategy,
	pool worker.Pool,
	delegate CheckDelegate,
	client worker.Client,
) *CheckStep {
	return &CheckStep{
		planID:                planID,
		plan:                  plan,
		metadata:              metadata,
		resourceFactory:       resourceFactory,
		resourceConfigFactory: resourceConfigFactory,
		containerMetadata:     containerMetadata,
		pool:                  pool,
		strategy:              strategy,
		delegate:              delegate,
		workerClient:          client,
	}
}

func (step *CheckStep) Run(ctx context.Context, state RunState) error {
	ctx, span := tracing.StartSpan(ctx, "check", tracing.Attrs{
		"team":     step.metadata.TeamName,
		"pipeline": step.metadata.PipelineName,
		"job":      step.metadata.JobName,
		"build":    step.metadata.BuildName,
		"name":     step.plan.Name,
	})

	err := step.run(ctx, state)
	tracing.End(span, err)

	return err
}

func (step *CheckStep) run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("check-step", lager.Data{
		"step-name": step.plan.Name,
	})

	variables := step.delegate.Variables()

	source, err := creds.NewSource(variables, step.plan.Source).Evaluate()
	if err != nil {
		return fmt.Errorf("resource config creds evaluation: %w", err)
	}

	resourceTypes, err := creds.NewVersionedResourceTypes(variables, step.plan.VersionedResourceTypes).Evaluate()
	if err != nil {
		return fmt.Errorf("resource types creds evaluation: %w", err)
	}

	timeout, err := time.ParseDuration(step.plan.Timeout)
	if err != nil {
		return fmt.Errorf("timeout parse: %w", err)
	}

	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: step.plan.Type,
		},
		BindMounts: []worker.BindMountSource{
			&worker.CertsVolumeMount{Logger: logger},
		},
		Tags:   step.plan.Tags,
		TeamID: step.metadata.TeamID,
		Env:    step.metadata.Env(),
	}
	tracing.Inject(ctx, &containerSpec)

	workerSpec := worker.WorkerSpec{
		ResourceType:  step.plan.Type,
		Tags:          step.plan.Tags,
		ResourceTypes: resourceTypes,
		TeamID:        step.metadata.TeamID,
	}

	expires := db.ContainerOwnerExpiries{
		Min: 5 * time.Minute,
		Max: 1 * time.Hour,
	}

	// XXX(check-refactor): this might possibly get GC'd - do we need an owner or
	// a use?
	resourceConfig, err := step.resourceConfigFactory.FindOrCreateResourceConfig(step.plan.Type, source, resourceTypes)
	if err != nil {
		return fmt.Errorf("create resource config: %w", err)
	}

	owner := db.NewResourceConfigCheckSessionContainerOwner(
		resourceConfig.ID(),
		resourceConfig.OriginBaseResourceType().ID,
		expires,
	)

	checkable := step.resourceFactory.NewResource(
		source,
		nil,
		step.plan.FromVersion,
	)

	imageSpec := worker.ImageFetcherSpec{
		ResourceTypes: resourceTypes,
		Delegate:      step.delegate,
	}

	result, err := step.workerClient.RunCheckStep(
		ctx,
		logger,
		owner,
		containerSpec,
		workerSpec,
		step.strategy,

		step.containerMetadata,
		imageSpec,

		timeout,
		checkable,
	)
	if err != nil {
		return fmt.Errorf("run check step: %w", err)
	}

	err = step.delegate.SaveVersions(db.NewSpanContext(ctx), resourceConfig, result.Versions)
	if err != nil {
		return fmt.Errorf("save versions: %w", err)
	}

	step.succeeded = true

	return nil
}

func (step *CheckStep) Succeeded() bool {
	return step.succeeded
}
