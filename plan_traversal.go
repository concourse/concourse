package atc

type PlanTraversal struct {
	TraverseFunc func(plan *Plan)
}

func (pt PlanTraversal) Traverse(plan *Plan) {
	pt.TraverseFunc(plan)

	switch {
	case plan.Aggregate != nil:
		for i := range *plan.Aggregate {
			pt.Traverse(&(*plan.Aggregate)[i])
		}

	case plan.Do != nil:
		for i := range *plan.Do {
			pt.Traverse(&(*plan.Do)[i])
		}

	case plan.Timeout != nil:
		pt.Traverse(&plan.Timeout.Step)

	case plan.Try != nil:
		pt.Traverse(&plan.Try.Step)

	case plan.OnSuccess != nil:
		pt.Traverse(&plan.OnSuccess.Step)
		pt.Traverse(&plan.OnSuccess.Next)

	case plan.OnFailure != nil:
		pt.Traverse(&plan.OnFailure.Step)
		pt.Traverse(&plan.OnFailure.Next)

	case plan.Ensure != nil:
		pt.Traverse(&plan.Ensure.Step)
		pt.Traverse(&plan.Ensure.Next)

	case plan.Retry != nil:
		for i := range *plan.Retry {
			pt.Traverse(&(*plan.Retry)[i])
		}
	}
}
