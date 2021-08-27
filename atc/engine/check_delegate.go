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
) exec.CheckDelegate {
	return &checkDelegate{
		BuildStepDelegate: NewBuildStepDelegate(build, plan.ID, state, clock, policyChecker),

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

	// stashed away just so we don't have to query it multiple times
	cachedPipeline db.Pipeline

	limiter RateLimiter
}

func (d *checkDelegate) Initializing(logger lager.Logger) {
	err := d.build.SaveEvent(event.InitializeCheck{
		Origin: d.eventOrigin,
		Time:   time.Now().Unix(),
		Name:   d.plan.Name,
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-check-event", err)
		return
	}

	logger.Info("initializing")
}

func (d *checkDelegate) FindOrCreateScope(config db.ResourceConfig) (db.ResourceConfigScope, error) {
	var resourceIDPtr *int
	if d.plan.Resource != "" {
		pipeline, err := d.pipeline()
		if err != nil {
			return nil, fmt.Errorf("get pipeline: %w", err)
		}

		resourceID, found, err := pipeline.ResourceID(d.plan.Resource)
		if err != nil {
			return nil, fmt.Errorf("get resource: %w", err)
		}
		if !found {
			return nil, fmt.Errorf("resource '%s' deleted", d.plan.Resource)
		}
		resourceIDPtr = &resourceID
	}

	scope, err := config.FindOrCreateScope(resourceIDPtr)
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
	logger := lagerctx.FromContext(ctx)

	if !d.plan.SkipInterval {
		if d.plan.Interval.Never == true {
			// exit early if user specified to never run periodic checks
			return nil, false, nil
		} else if d.plan.Resource != "" {
			// rate limit periodic resource checks so worker load (plus load on
			// external services) isn't too spiky. note that we don't rate limit
			// resource type or prototype checks, because they are created every time a
			// resource is used (rather than periodically).
			err := d.limiter.Wait(ctx)
			if err != nil {
				return nil, false, fmt.Errorf("rate limit: %w", err)
			}
		}
	}

	interval := d.plan.Interval.Interval

	var lock lock.Lock = lock.NoopLock{}
	if d.plan.IsPeriodic() {
		for {
			lastCheck, err := scope.LastCheck()
			if err != nil {
				return nil, false, err
			}

			if d.plan.SkipInterval { // if the check was manually triggered
				// If the check plan does not provide a from version
				if d.plan.FromVersion == nil {
					// If the last check succeeded and the check was created before the last
					// check start time, then don't run
					// This is so that we will avoid running redundant mnaual checks
					if lastCheck.Succeeded && d.build.CreateTime().Before(lastCheck.StartTime) {
						return nil, false, nil
					}
				}
			} else {
				// For periodic checks, if the current time is before the end of the last
				// check + the interval, do not run
				if d.clock.Now().Before(lastCheck.EndTime.Add(interval)) {
					return nil, false, nil
				}
			}

			var acquired bool
			lock, acquired, err = scope.AcquireResourceCheckingLock(logger)
			if err != nil {
				return nil, false, fmt.Errorf("acquire lock: %w", err)
			}

			if acquired {
				break
			}

			select {
			case <-ctx.Done():
				return nil, false, ctx.Err()
			case <-d.clock.After(time.Second):
			}
		}
	} else {
		lastCheck, err := scope.LastCheck()
		if err != nil {
			return nil, false, err
		}

		// If last check succeeded and the end of the last check is after the start
		// of this check, then don't run
		if lastCheck.Succeeded && lastCheck.EndTime.After(d.build.StartTime()) {
			return nil, false, nil
		}
	}

	return lock, true, nil
}

func (d *checkDelegate) PointToCheckedConfig(scope db.ResourceConfigScope) error {
	if d.plan.Resource != "" {
		pipeline, err := d.pipeline()
		if err != nil {
			return fmt.Errorf("get pipeline: %w", err)
		}

		err = pipeline.SetResourceConfigScopeForResource(d.plan.Resource, scope)
		if err != nil {
			return fmt.Errorf("set resource scope: %w", err)
		}
	}

	if d.plan.ResourceType != "" {
		pipeline, err := d.pipeline()
		if err != nil {
			return fmt.Errorf("get pipeline: %w", err)
		}

		err = pipeline.SetResourceConfigScopeForResourceType(d.plan.ResourceType, scope)
		if err != nil {
			return fmt.Errorf("set resource type scope: %w", err)
		}
	}

	if d.plan.Prototype != "" {
		pipeline, err := d.pipeline()
		if err != nil {
			return fmt.Errorf("get pipeline: %w", err)
		}

		err = pipeline.SetResourceConfigScopeForPrototype(d.plan.Prototype, scope)
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
