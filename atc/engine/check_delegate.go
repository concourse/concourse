package engine

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/worker"
)

//counterfeiter:generate . RateLimiter
type RateLimiter interface {
	Wait(context.Context) error
}

func NewCheckDelegate(
	build db.Build,
	plan atc.Plan,
	state exec.RunState,
	clock clock.Clock,
	limiter RateLimiter,
	policyChecker policy.Checker,
	artifactSourcer worker.ArtifactSourcer,
) exec.CheckDelegate {
	return &checkDelegate{
		BuildStepDelegate: NewBuildStepDelegate(build, plan.ID, state, clock, policyChecker, artifactSourcer),

		build:       build,
		plan:        plan.Check,
		eventOrigin: event.Origin{ID: event.OriginID(plan.ID)},
		clock:       clock,

		limiter: limiter,
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
	cachedPrototype    db.Prototype

	limiter RateLimiter
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

// WaitToRun decides if a check should really run or just reuse a previous result, and acquires
// a check lock accordingly. There are three types of checks, each reflects to a different behavior:
// 1) A Lidar triggered checks should always run once reach to next check time;
// 2) A manually triggered checks may reuse a previous result if the last check succeeded and began
// later than the current check build's create time.
// 3) A step embedded check may reuse a previous step if the last check succeeded and finished later
// than the current build started.
func (d *checkDelegate) WaitToRun(ctx context.Context, scope db.ResourceConfigScope) (lock.Lock, bool, error) {
	logger := lagerctx.FromContext(ctx).Session("wait-to-run")

	// rate limit lidar-scheduled resource checks so worker load (plus load on
	// external services) isn't too spiky. note that we don't rate limit
	// resource type or prototype checks, since they don't run periodically -
	// they only run as needed (when used by a resource)
	if !d.build.IsManuallyTriggered() && d.plan.Resource != "" {
		err := d.limiter.Wait(lagerctx.NewContext(ctx, logger))
		if err != nil {
			return nil, false, fmt.Errorf("rate limit: %w", err)
		}
	}

	var err error

	var interval time.Duration
	if d.plan.Interval != "" {
		interval, err = time.ParseDuration(d.plan.Interval)
		if err != nil {
			return nil, false, err
		}
		logger.Debug("interval", lager.Data{"interval": interval})
	}

	var lock lock.Lock = lock.NoopLock{}
	if d.plan.Resource != "" {
		logger.Debug("will-acquire-lock")
		for {
			var acquired bool
			lock, acquired, err = scope.AcquireResourceCheckingLock(logger)
			if err != nil {
				return nil, false, fmt.Errorf("acquire lock: %w", err)
			}

			logger.Debug("acquiring-lock", lager.Data{"acquired": acquired})
			if acquired {
				break
			}

			d.clock.Sleep(time.Second)
		}
	}

	lastCheck, err := scope.LastCheck()
	if err != nil {
		if releaseErr := lock.Release(); releaseErr != nil {
			logger.Error("failed-to-release-lock", releaseErr)
		}
		return nil, false, err
	}

	shouldRun := false
	// TODO: what does this do?
	//if d.plan.Resource == "" && d.plan.ResourceType == "" && d.plan.Prototype == "" {
	//	// anonymous checks (image_resources) shouldn't run if they started
	//	// before the last (successful) check ended (e.g. multiple tasks in a
	//	// build use the same image_resource, so only check one of them) ???
	//	shouldRun = !lastCheck.Succeeded || d.build.StartTime().After(lastCheck.EndTime)
	//} else if d.build.IsManuallyTriggered() {
	if d.build.IsManuallyTriggered() {
		// If a manually triggered check takes a from version, then it should be run.
		if d.plan.FromVersion != nil {
			shouldRun = true
		} else {
			// manually triggered checks ignore the interval, but won't run if
			// there is another check that has started after this check was
			// created (e.g. if multiple webhooks come in for a git repo that's
			// common to several pipelines at around the same time, we should
			// only run the check once).
			shouldRun = !lastCheck.Succeeded || d.build.CreateTime().After(lastCheck.StartTime)
		}
	} else {
		shouldRun = d.clock.Now().After(lastCheck.EndTime.Add(interval))
	}

	logger.Debug("should-run?", lager.Data{"result": shouldRun})

	// XXX(check-refactor): we could add an else{} case and potentially sleep
	// here until runAt is reached.
	//
	// then the check build queueing logic is to just make sure there's a build
	// running for every resource, without having to check if intervals have
	// elapsed.
	//
	// this could be expanded upon to short-circuit the waiting with events
	// triggered by webhooks so that webhooks are super responsive: rather than
	// queueing a build, it would just wake up a goroutine.

	if !shouldRun {
		err := lock.Release()
		if err != nil {
			return nil, false, fmt.Errorf("release lock: %w", err)
		}

		return nil, false, nil
	}

	return lock, true, nil
}

func (d *checkDelegate) PointToCheckedConfig(scope db.ResourceConfigScope) error {
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

	prototype, found, err := d.prototype()
	if err != nil {
		return fmt.Errorf("get prototype: %w", err)
	}

	if found {
		err := prototype.SetResourceConfigScope(scope)
		if err != nil {
			return fmt.Errorf("set prototype scope: %w", err)
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

func (d *checkDelegate) prototype() (db.Prototype, bool, error) {
	if d.plan.Prototype == "" {
		return nil, false, nil
	}

	if d.cachedPrototype != nil {
		return d.cachedPrototype, true, nil
	}

	pipeline, err := d.pipeline()
	if err != nil {
		return nil, false, err
	}

	prototype, found, err := pipeline.Prototype(d.plan.Prototype)
	if err != nil {
		return nil, false, fmt.Errorf("get pipeline prototype: %w", err)
	}

	if !found {
		return nil, false, fmt.Errorf("prototype '%s' deleted", d.plan.Prototype)
	}

	d.cachedPrototype = prototype

	return d.cachedPrototype, true, nil
}
