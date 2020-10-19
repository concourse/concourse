package exec

import (
	"context"
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
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
	delegateFactory       CheckDelegateFactory
	workerClient          worker.Client
	defaultCheckTimeout   time.Duration
}

//go:generate counterfeiter . CheckDelegateFactory

type CheckDelegateFactory interface {
	CheckDelegate(state RunState) CheckDelegate
}

//go:generate counterfeiter . CheckDelegate

type CheckDelegate interface {
	BuildStepDelegate

	FindOrCreateScope(db.ResourceConfig) (db.ResourceConfigScope, error)
	WaitToRun(context.Context, db.ResourceConfigScope) (lock.Lock, bool, error)
	PointToCheckedConfig(db.ResourceConfigScope) error
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
	delegateFactory CheckDelegateFactory,
	client worker.Client,
	defaultCheckTimeout time.Duration,
) Step {
	return &CheckStep{
		planID:                planID,
		plan:                  plan,
		metadata:              metadata,
		resourceFactory:       resourceFactory,
		resourceConfigFactory: resourceConfigFactory,
		containerMetadata:     containerMetadata,
		pool:                  pool,
		strategy:              strategy,
		delegateFactory:       delegateFactory,
		workerClient:          client,
		defaultCheckTimeout:   defaultCheckTimeout,
	}
}

func (step *CheckStep) Run(ctx context.Context, state RunState) (bool, error) {
	attrs := tracing.Attrs{
		"name": step.plan.Name,
	}

	if step.plan.Resource != "" {
		attrs["resource"] = step.plan.Resource
	}

	if step.plan.ResourceType != "" {
		attrs["resource_type"] = step.plan.ResourceType
	}

	delegate := step.delegateFactory.CheckDelegate(state)
	ctx, span := delegate.StartSpan(ctx, "check", attrs)

	ok, err := step.run(ctx, state, delegate)
	tracing.End(span, err)

	return ok, err
}

func (step *CheckStep) run(ctx context.Context, state RunState, delegate CheckDelegate) (bool, error) {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("check-step", lager.Data{
		"step-name": step.plan.Name,
	})

	delegate.Initializing(logger)

	timeout := step.defaultCheckTimeout
	if step.plan.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(step.plan.Timeout)
		if err != nil {
			return false, fmt.Errorf("parse timeout: %w", err)
		}
	}

	source, err := creds.NewSource(state, step.plan.Source).Evaluate()
	if err != nil {
		return false, fmt.Errorf("resource config creds evaluation: %w", err)
	}

	resourceTypes, err := creds.NewVersionedResourceTypes(state, step.plan.VersionedResourceTypes).Evaluate()
	if err != nil {
		return false, fmt.Errorf("resource types creds evaluation: %w", err)
	}

	resourceConfig, err := step.resourceConfigFactory.FindOrCreateResourceConfig(step.plan.Type, source, resourceTypes)
	if err != nil {
		return false, fmt.Errorf("create resource config: %w", err)
	}

	// XXX(check-refactor): we should remove scopes as soon as it's safe to do
	// so, i.e. global resources is on by default. i think this can be done when
	// time resource becomes time var source (resolving thundering herd problem)
	// and IAM is handled via var source prototypes (resolving unintentionally
	// shared history problem)
	scope, err := delegate.FindOrCreateScope(resourceConfig)
	if err != nil {
		return false, fmt.Errorf("create resource config scope: %w", err)
	}

	lock, run, err := delegate.WaitToRun(ctx, scope)
	if err != nil {
		return false, fmt.Errorf("wait: %w", err)
	}

	if run {
		defer func() {
			err := lock.Release()
			if err != nil {
				logger.Error("failed-to-release-lock", err)
			}
		}()

		fromVersion := step.plan.FromVersion
		if fromVersion == nil {
			latestVersion, found, err := scope.LatestVersion()
			if err != nil {
				return false, fmt.Errorf("get latest version: %w", err)
			}

			if found {
				fromVersion = atc.Version(latestVersion.Version())
			}
		}

		metric.Metrics.ChecksStarted.Inc()

		_, err = scope.UpdateLastCheckStartTime()
		if err != nil {
			return false, fmt.Errorf("update check end time: %w", err)
		}

		result, err := step.runCheck(ctx, logger, delegate, timeout, resourceConfig, source, resourceTypes, fromVersion)
		if err != nil {
			metric.Metrics.ChecksFinishedWithError.Inc()

			if pointErr := delegate.PointToCheckedConfig(scope); pointErr != nil {
				return false, fmt.Errorf("update resource config scope: %w", pointErr)
			}

			var scriptErr runtime.ErrResourceScriptFailed
			if errors.As(err, &scriptErr) {
				delegate.Finished(logger, false)
				return false, nil
			}

			return false, fmt.Errorf("run check: %w", err)
		}

		metric.Metrics.ChecksFinishedWithSuccess.Inc()

		err = scope.SaveVersions(db.NewSpanContext(ctx), result.Versions)
		if err != nil {
			return false, fmt.Errorf("save versions: %w", err)
		}

		if len(result.Versions) > 0 {
			state.StoreResult(step.planID, result.Versions[len(result.Versions)-1])
		}

		_, err = scope.UpdateLastCheckEndTime()
		if err != nil {
			return false, fmt.Errorf("update check end time: %w", err)
		}
	} else {
		latestVersion, found, err := scope.LatestVersion()
		if err != nil {
			return false, fmt.Errorf("get latest version: %w", err)
		}

		if found {
			state.StoreResult(step.planID, atc.Version(latestVersion.Version()))
		}
	}

	err = delegate.PointToCheckedConfig(scope)
	if err != nil {
		return false, fmt.Errorf("update resource config scope: %w", err)
	}

	delegate.Finished(logger, true)

	return true, nil
}

func (step *CheckStep) runCheck(
	ctx context.Context,
	logger lager.Logger,
	delegate CheckDelegate,
	timeout time.Duration,
	resourceConfig db.ResourceConfig,
	source atc.Source,
	resourceTypes atc.VersionedResourceTypes,
	fromVersion atc.Version,
) (worker.CheckResult, error) {
	workerSpec := worker.WorkerSpec{
		Tags:   step.plan.Tags,
		TeamID: step.metadata.TeamID,
	}

	var imageSpec worker.ImageSpec
	resourceType, found := step.plan.VersionedResourceTypes.Lookup(step.plan.Type)
	if found {
		image := atc.ImageResource{
			Name:    resourceType.Name,
			Type:    resourceType.Type,
			Source:  resourceType.Source,
			Params:  resourceType.Params,
			Version: resourceType.Version,
		}

		types := step.plan.VersionedResourceTypes.Without(step.plan.Type)

		var err error
		imageSpec, err = delegate.FetchImage(ctx, image, types, resourceType.Privileged)
		if err != nil {
			return worker.CheckResult{}, err
		}
	} else {
		imageSpec.ResourceType = step.plan.Type
		workerSpec.ResourceType = step.plan.Type
	}

	containerSpec := worker.ContainerSpec{
		ImageSpec: imageSpec,
		BindMounts: []worker.BindMountSource{
			&worker.CertsVolumeMount{Logger: logger},
		},
		Tags:   step.plan.Tags,
		TeamID: step.metadata.TeamID,
		Env:    step.metadata.Env(),
	}
	tracing.Inject(ctx, &containerSpec)

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

	processSpec := runtime.ProcessSpec{
		Path:         "/opt/resource/check",
		StdoutWriter: delegate.Stdout(),
		StderrWriter: delegate.Stderr(),
	}

	return step.workerClient.RunCheckStep(
		ctx,
		logger,
		owner,
		containerSpec,
		workerSpec,
		step.strategy,
		step.containerMetadata,
		processSpec,
		delegate,
		checkable,
		timeout,
	)
}
