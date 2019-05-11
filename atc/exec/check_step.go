package exec

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
)

type CheckStep struct {
	planID            atc.PlanID
	plan              atc.CheckPlan
	metadata          StepMetadata
	containerMetadata db.ContainerMetadata
	resourceFactory   resource.ResourceFactory
	strategy          worker.ContainerPlacementStrategy
	pool              worker.Pool
	delegate          CheckDelegate
	succeeded         bool
}

type CheckDelegate interface {
}

func NewCheckStep(
	planID atc.PlanID,
	plan atc.CheckPlan,
	metadata StepMetadata,
	containerMetadata db.ContainerMetadata,
	resourceFactory resource.ResourceFactory,
	strategy worker.ContainerPlacementStrategy,
	pool worker.Pool,
	delegate CheckDelegate,
) *CheckStep {
	return &CheckStep{
		planID:            planID,
		plan:              plan,
		metadata:          metadata,
		containerMetadata: containerMetadata,
		resourceFactory:   resourceFactory,
		pool:              pool,
		strategy:          strategy,
		delegate:          delegate,
	}
}

func (step *CheckStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("check-step", lager.Data{
		"step-name": step.plan.Name,
	})

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

	workerSpec := worker.WorkerSpec{
		ResourceType:  step.plan.Type,
		Tags:          step.plan.Tags,
		ResourceTypes: step.plan.VersionedResourceTypes,
		TeamID:        step.metadata.TeamID,
	}

	owner := db.NewResourceConfigCheckSessionContainerOwner(
		step.metadata.ResourceConfigID,
		step.metadata.BaseResourceTypeID,
		db.ContainerOwnerExpiries{
			Min: 5 * time.Minute,
			Max: 1 * time.Hour,
		},
	)

	containerMetadata := db.ContainerMetadata{
		Type: db.ContainerTypeCheck,
	}

	chosenWorker, err := step.pool.FindOrChooseWorkerForContainer(
		ctx,
		logger,
		owner,
		containerSpec,
		containerMetadata,
		workerSpec,
		worker.NewRandomPlacementStrategy(),
	)
	if err != nil {
		return err
	}

	container, err := chosenWorker.FindOrCreateContainer(
		ctx,
		logger,
		worker.NoopImageFetchingDelegate{},
		owner,
		containerSpec,
		step.plan.VersionedResourceTypes,
	)
	if err != nil {
		return err
	}

	timeout, err := time.ParseDuration(step.plan.Timeout)
	if err != nil {
		return err
	}

	deadline, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	checkable := step.resourceFactory.NewResourceForContainer(container)

	versions, err := checkable.Check(deadline, step.plan.Source, *step.plan.FromVersion)
	if err != nil {
		if err == context.DeadlineExceeded {
			return fmt.Errorf("Timed out after %v while checking for new versions", timeout)
		}
		return err
	}

	step.succeeded = true

	return nil
}

func (step *CheckStep) Succeeded() bool {
	return step.succeeded
}
