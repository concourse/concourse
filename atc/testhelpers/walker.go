package testhelpers

import "github.com/concourse/concourse/atc"

type PlanWalker func(atc.Plan) (atc.Plan, error)

// Walks the plan tree structure calling walker for each encountered Plan,
// including the root.
func WalkPlan(plan atc.Plan, walker PlanWalker) (atc.Plan, error) {
	plan, err := walker(plan)
	if err != nil {
		return atc.Plan{}, err
	}

	if plan.Aggregate != nil {
		for i, p := range *plan.Aggregate {
			(*plan.Aggregate)[i], err = WalkPlan(p, walker)
			if err != nil {
				return atc.Plan{}, err
			}
		}
	}

	if plan.InParallel != nil {
		for i, p := range plan.InParallel.Steps {
			plan.InParallel.Steps[i], err = WalkPlan(p, walker)
			if err != nil {
				return atc.Plan{}, err
			}
		}
	}

	if plan.Do != nil {
		for i, p := range *plan.Do {
			(*plan.Do)[i], err = WalkPlan(p, walker)
			if err != nil {
				return atc.Plan{}, err
			}
		}
	}

	if plan.Get != nil {
		plan, err = walker(plan)
		if err != nil {
			return atc.Plan{}, err
		}
	}

	if plan.Put != nil {
		plan, err = walker(plan)
		if err != nil {
			return atc.Plan{}, err
		}
	}

	if plan.Check != nil {
		plan, err = walker(plan)
		if err != nil {
			return atc.Plan{}, err
		}
	}

	if plan.Task != nil {
		plan, err = walker(plan)
		if err != nil {
			return atc.Plan{}, err
		}
	}

	if plan.SetPipeline != nil {
		plan, err = walker(plan)
		if err != nil {
			return atc.Plan{}, err
		}
	}

	if plan.LoadVar != nil {
		plan, err = walker(plan)
		if err != nil {
			return atc.Plan{}, err
		}
	}

	if plan.OnAbort != nil {
		plan.OnAbort.Step, err = WalkPlan(plan.OnAbort.Step, walker)
		if err != nil {
			return atc.Plan{}, err
		}

		plan.OnAbort.Next, err = WalkPlan(plan.OnAbort.Next, walker)
		if err != nil {
			return atc.Plan{}, err
		}
	}

	if plan.OnError != nil {
		plan.OnError.Step, err = WalkPlan(plan.OnError.Step, walker)
		if err != nil {
			return atc.Plan{}, err
		}

		plan.OnError.Next, err = WalkPlan(plan.OnError.Next, walker)
		if err != nil {
			return atc.Plan{}, err
		}
	}

	if plan.Ensure != nil {
		plan.Ensure.Step, err = WalkPlan(plan.Ensure.Step, walker)
		if err != nil {
			return atc.Plan{}, err
		}

		plan.Ensure.Next, err = WalkPlan(plan.Ensure.Next, walker)
		if err != nil {
			return atc.Plan{}, err
		}
	}

	if plan.OnSuccess != nil {
		plan.OnSuccess.Step, err = WalkPlan(plan.OnSuccess.Step, walker)
		if err != nil {
			return atc.Plan{}, err
		}

		plan.OnSuccess.Next, err = WalkPlan(plan.OnSuccess.Next, walker)
		if err != nil {
			return atc.Plan{}, err
		}
	}

	if plan.OnFailure != nil {
		plan.OnFailure.Step, err = WalkPlan(plan.OnFailure.Step, walker)
		if err != nil {
			return atc.Plan{}, err
		}

		plan.OnFailure.Next, err = WalkPlan(plan.OnFailure.Next, walker)
		if err != nil {
			return atc.Plan{}, err
		}
	}

	if plan.Try != nil {
		plan.Try.Step, err = WalkPlan(plan.Try.Step, walker)
		if err != nil {
			return atc.Plan{}, err
		}
	}

	if plan.Timeout != nil {
		plan.Timeout.Step, err = WalkPlan(plan.Timeout.Step, walker)
		if err != nil {
			return atc.Plan{}, err
		}
	}

	if plan.Retry != nil {
		for i, p := range *plan.Retry {
			(*plan.Retry)[i], err = WalkPlan(p, walker)
			if err != nil {
				return atc.Plan{}, err
			}
		}
	}

	return plan, nil
}
