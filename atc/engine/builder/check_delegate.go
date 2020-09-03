package builder

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/vars"
)

func NewCheckDelegate(build db.Build, plan atc.Plan, buildVars *vars.BuildVariables, clock clock.Clock) exec.CheckDelegate {
	return &checkDelegate{
		BuildStepDelegate: NewBuildStepDelegate(build, plan.ID, buildVars, clock),

		build:       build,
		plan:        plan.Check,
		eventOrigin: event.Origin{ID: event.OriginID(plan.ID)},
		clock:       clock,
	}
}

type checkDelegate struct {
	exec.BuildStepDelegate

	build       db.Build
	plan        *atc.CheckPlan
	eventOrigin event.Origin
	clock       clock.Clock

	// stashed away just so we don't have to query them multiple times
	cachedPipeline     db.Pipeline
	cachedResource     db.Resource
	cachedResourceType db.ResourceType
}

func (d *checkDelegate) FindOrCreateScope(config db.ResourceConfig) (db.ResourceConfigScope, error) {
	resource, _, err := d.resource()
	if err != nil {
		return nil, fmt.Errorf("get resource: %w", err)
	}

	scope, err := config.FindOrCreateScope(resource) // ignore found, nil is ok
	if err != nil {
		return nil, fmt.Errorf("find or create scope: %w", err)
	}

	return scope, nil
}

func (d *checkDelegate) WaitAndRun(ctx context.Context, scope db.ResourceConfigScope) (lock.Lock, bool, error) {
	logger := lagerctx.FromContext(ctx)

	var err error

	var interval time.Duration
	if d.plan.Interval != "" {
		interval, err = time.ParseDuration(d.plan.Interval)
		if err != nil {
			return nil, false, err
		}
	}

	var lock lock.Lock
	for {
		var acquired bool
		lock, acquired, err = scope.AcquireResourceCheckingLock(logger)
		if err != nil {
			return nil, false, fmt.Errorf("acquire lock: %w", err)
		}

		if acquired {
			break
		}

		d.clock.Sleep(time.Second)
	}

	// XXX(check-refactor): one interesting thing we could do here is literally
	// wait until the interval elapses.
	//
	// then all of checking would be modeled as rate limiting. we'd just make
	// sure a build was running for each resource and keep queueing up another
	// when the last one finishes.

	end, err := scope.LastCheckEndTime()
	if err != nil {
		if releaseErr := lock.Release(); releaseErr != nil {
			logger.Error("failed-to-release-lock", releaseErr)
		}

		return nil, false, fmt.Errorf("get last check end time: %w", err)
	}

	runAt := end.Add(interval)

	shouldRun := false
	if d.build.IsManuallyTriggered() {
		// do not delay manually triggered checks (or builds)
		shouldRun = true
	} else if !d.clock.Now().Before(runAt) {
		// run if we're past the last check end time
		shouldRun = true
	} else {
		// XXX(check-refactor): we could potentially sleep here until runAt is
		// reached.
		//
		// then the check build queueing logic is to just make sure there's a build
		// running for every resource, without having to check if intervals have
		// elapsed.
		//
		// this could be expanded upon to short-circuit the waiting with events
		// triggered webhooks so that webhooks are super responsive - rather than
		// queueing a build, it would just wake up a goroutine
	}

	if !shouldRun {
		err := lock.Release()
		if err != nil {
			return nil, false, fmt.Errorf("release lock: %w", err)
		}

		return nil, false, nil
	}

	// XXX(check-refactor): enforce global rate limiting

	return lock, true, nil
}

func (d *checkDelegate) PointToSavedVersions(scope db.ResourceConfigScope) error {
	resource, found, err := d.resource()
	if err != nil {
		return fmt.Errorf("get resource: %w", err)
	}

	if found {
		err := resource.SetResourceConfigScope(scope)
		if err != nil {
			return fmt.Errorf("set resource scope: %w", err)
		}
	}

	resourceType, found, err := d.resourceType()
	if err != nil {
		return fmt.Errorf("get resource type: %w", err)
	}

	if found {
		err := resourceType.SetResourceConfigScope(scope)
		if err != nil {
			return fmt.Errorf("set resource type scope: %w", err)
		}
	}

	return nil
}

func (d *checkDelegate) pipeline() (db.Pipeline, error) {
	if d.cachedPipeline != nil {
		return d.cachedPipeline, nil
	}

	pipeline, found, err := d.build.Pipeline()
	if err != nil {
		return nil, fmt.Errorf("get build pipeline: %w", err)
	}

	if !found {
		return nil, fmt.Errorf("pipeline not found")
	}

	d.cachedPipeline = pipeline

	return d.cachedPipeline, nil
}

func (d *checkDelegate) resource() (db.Resource, bool, error) {
	if d.plan.Resource == "" {
		return nil, false, nil
	}

	if d.cachedResource != nil {
		return d.cachedResource, true, nil
	}

	pipeline, err := d.pipeline()
	if err != nil {
		return nil, false, err
	}

	resource, found, err := pipeline.Resource(d.plan.Resource)
	if err != nil {
		return nil, false, fmt.Errorf("get pipeline resource: %w", err)
	}

	if !found {
		return nil, false, fmt.Errorf("resource '%s' deleted", d.plan.Resource)
	}

	d.cachedResource = resource

	return d.cachedResource, true, nil
}

func (d *checkDelegate) resourceType() (db.ResourceType, bool, error) {
	if d.plan.ResourceType == "" {
		return nil, false, nil
	}

	if d.cachedResourceType != nil {
		return d.cachedResourceType, true, nil
	}

	pipeline, err := d.pipeline()
	if err != nil {
		return nil, false, err
	}

	resourceType, found, err := pipeline.ResourceType(d.plan.ResourceType)
	if err != nil {
		return nil, false, fmt.Errorf("get pipeline resource type: %w", err)
	}

	if !found {
		return nil, false, fmt.Errorf("resource type '%s' deleted", d.plan.ResourceType)
	}

	d.cachedResourceType = resourceType

	return d.cachedResourceType, true, nil
}
