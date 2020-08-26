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
	"github.com/concourse/concourse/atc/db/lock"
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

	FindOrCreateScope(db.ResourceConfig) (db.ResourceConfigScope, error)
	WaitAndRun(context.Context, db.ResourceConfigScope) (lock.Lock, bool, error)
	PointToSavedVersions(db.ResourceConfigScope) error
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

	timeout, err := time.ParseDuration(step.plan.Timeout)
	if err != nil {
		return fmt.Errorf("parse timeout: %w", err)
	}

	variables := step.delegate.Variables()

	source, err := creds.NewSource(variables, step.plan.Source).Evaluate()
	if err != nil {
		return fmt.Errorf("resource config creds evaluation: %w", err)
	}

	resourceTypes, err := creds.NewVersionedResourceTypes(variables, step.plan.VersionedResourceTypes).Evaluate()
	if err != nil {
		return fmt.Errorf("resource types creds evaluation: %w", err)
	}

	resourceConfig, err := step.resourceConfigFactory.FindOrCreateResourceConfig(step.plan.Type, source, resourceTypes)
	if err != nil {
		return fmt.Errorf("create resource config: %w", err)
	}

	// XXX(global-resources): remove this when we don't have to worry about
	// global resources anymore (i.e. when time resource becomes time var source
	// and IAM is handled via var source prototypes)
	scope, err := step.delegate.FindOrCreateScope(resourceConfig)
	if err != nil {
		return fmt.Errorf("create resource config scope: %w", err)
	}

	lock, run, err := step.delegate.WaitAndRun(ctx, scope)
	if err != nil {
		return fmt.Errorf("wait: %w", err)
	}

	if run {
		defer func() {
			err := lock.Release()
			if err != nil {
				logger.Error("failed-to-release-lock", err)
			}
		}()

		// get the latest version AFTER waiting!
		//
		// XXX(check-refactor): it's actually cool that we don't get the latest
		// version until here, because it means if someone else checked while we
		// were waiting we'll pick up the new version
		fromVersion := step.plan.FromVersion
		if fromVersion == nil {
			latestVersion, found, err := scope.LatestVersion()
			if err != nil {
				return fmt.Errorf("get latest version: %w", err)
			}

			if found {
				fromVersion = atc.Version(latestVersion.Version())
			}
		}

		_, err = scope.UpdateLastCheckStartTime()
		if err != nil {
			return fmt.Errorf("update check end time: %w", err)
		}

		result, err := step.runCheck(ctx, logger, timeout, resourceConfig, source, resourceTypes, fromVersion)
		if setErr := scope.SetCheckError(err); setErr != nil {
			logger.Error("failed-to-set-check-error", setErr)
		}
		if err != nil {
			return fmt.Errorf("run check: %w", err)
		}

		err = scope.SaveVersions(db.NewSpanContext(ctx), result.Versions)
		if err != nil {
			return fmt.Errorf("save versions: %w", err)
		}

		_, err = scope.UpdateLastCheckEndTime()
		if err != nil {
			return fmt.Errorf("update check end time: %w", err)
		}
	}

	// XXX(global-resources): set config instead of scope once scopes are
	// eliminated
	err = step.delegate.PointToSavedVersions(scope)
	if err != nil {
		return fmt.Errorf("update resource config scope: %w", err)
	}

	step.succeeded = true

	return nil
}

func (step *CheckStep) Succeeded() bool {
	return step.succeeded
}

func (step *CheckStep) runCheck(ctx context.Context, logger lager.Logger, timeout time.Duration, resourceConfig db.ResourceConfig, source atc.Source, resourceTypes atc.VersionedResourceTypes, fromVersion atc.Version) (worker.CheckResult, error) {
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

	// XXX(check-refactor): this can be turned into NewBuildStepContainerOwner
	// now, but we should understand the performance implications first - it'll
	// mean a lot more container churn
	owner := db.NewResourceConfigCheckSessionContainerOwner(
		resourceConfig.ID(),
		resourceConfig.OriginBaseResourceType().ID,
		expires,
	)

	checkable := step.resourceFactory.NewResource(
		source,
		nil,
		fromVersion,
	)

	imageSpec := worker.ImageFetcherSpec{
		ResourceTypes: resourceTypes,
		Delegate:      step.delegate,
	}

	return step.workerClient.RunCheckStep(
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
}
