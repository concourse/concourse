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
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
)

type CheckStep struct {
	planID            atc.PlanID
	plan              atc.CheckPlan
	metadata          StepMetadata
	containerMetadata db.ContainerMetadata
	secrets           creds.Secrets
	resourceFactory   resource.ResourceFactory
	strategy          worker.ContainerPlacementStrategy
	pool              worker.Pool
	delegate          CheckDelegate
	succeeded         bool
}

type CheckDelegate interface {
	BuildStepDelegate

	SaveVersions([]atc.Version) error
}

func NewCheckStep(
	planID atc.PlanID,
	plan atc.CheckPlan,
	metadata StepMetadata,
	containerMetadata db.ContainerMetadata,
	secrets creds.Secrets,
	resourceFactory resource.ResourceFactory,
	strategy worker.ContainerPlacementStrategy,
	pool worker.Pool,
	delegate CheckDelegate,
) *CheckStep {
	return &CheckStep{
		planID:            planID,
		plan:              plan,
		metadata:          metadata,
		secrets:           secrets,
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

	resourceTypes := step.plan.VersionedResourceTypes

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
		ResourceTypes: resourceTypes,
		TeamID:        step.metadata.TeamID,
	}

	expires := db.ContainerOwnerExpiries{
		Min: 5 * time.Minute,
		Max: 1 * time.Hour,
	}

	owner := db.NewResourceConfigCheckSessionContainerOwner(
		step.metadata.ResourceConfigID,
		step.metadata.BaseResourceTypeID,
		expires,
	)

	chosenWorker, err := step.pool.FindOrChooseWorkerForContainer(
		ctx,
		logger,
		owner,
		containerSpec,
		workerSpec,
		step.strategy,
	)
	if err != nil {
		logger.Error("failed-to-find-or-choose-worker", err)
		return err
	}

	container, err := chosenWorker.FindOrCreateContainer(
		ctx,
		logger,
		step.delegate,
		owner,
		step.containerMetadata,
		containerSpec,
		resourceTypes,
	)
	if err != nil {
		logger.Error("failed-to-find-or-create-container", err)
		return err
	}

	timeout, err := time.ParseDuration(step.plan.Timeout)
	if err != nil {
		logger.Error("failed-to-parse-timeout", err)
		return err
	}

	deadline, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	checkable := step.resourceFactory.NewResourceForContainer(container)

	versions, err := checkable.Check(deadline, step.plan.Source, step.plan.FromVersion)
	if err != nil {
		if err == context.DeadlineExceeded {
			return fmt.Errorf("Timed out after %v while checking for new versions", timeout)
		}
		return err
	}

	metric.ResourceCheck{
		PipelineName: step.metadata.PipelineName,
		ResourceName: step.plan.Name,
		TeamName:     step.metadata.TeamName,
		Success:      err == nil,
	}.Emit(logger)

	err = step.delegate.SaveVersions(versions)
	if err != nil {
		logger.Error("failed-to-save-versions", err)
		return err
	}

	step.succeeded = true

	return nil
}

func (step *CheckStep) Succeeded() bool {
	return step.succeeded
}
