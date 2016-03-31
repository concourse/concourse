package atc

type PlanTraverseFunc func(plan *Plan) error

type PlanTraversal interface {
	Traverse(plan *Plan) error
}

type planTraversal struct {
	traverseFunc PlanTraverseFunc
}

func NewPlanTraversal(f PlanTraverseFunc) PlanTraversal {
	return &planTraversal{traverseFunc: f}
}

func (pt *planTraversal) Traverse(plan *Plan) error {
	err := pt.traverseFunc(plan)
	if err != nil {
		return err
	}

	switch {
	case plan.Aggregate != nil:
		for i := range *plan.Aggregate {
			err = pt.Traverse(&(*plan.Aggregate)[i])
			if err != nil {
				return err
			}
		}

	case plan.Do != nil:
		for i := range *plan.Do {
			err = pt.Traverse(&(*plan.Do)[i])
			if err != nil {
				return err
			}
		}

	case plan.Timeout != nil:
		return pt.Traverse(&plan.Timeout.Step)

	case plan.Try != nil:
		return pt.Traverse(&plan.Try.Step)

	case plan.OnSuccess != nil:
		err = pt.Traverse(&plan.OnSuccess.Step)
		if err != nil {
			return err
		}
		return pt.Traverse(&plan.OnSuccess.Next)

	case plan.OnFailure != nil:
		err = pt.Traverse(&plan.OnFailure.Step)
		if err != nil {
			return err
		}
		return pt.Traverse(&plan.OnFailure.Next)

	case plan.Ensure != nil:
		err = pt.Traverse(&plan.Ensure.Step)
		if err != nil {
			return err
		}
		return pt.Traverse(&plan.Ensure.Next)

	case plan.Retry != nil:
		for i := range *plan.Retry {
			err = pt.Traverse(&(*plan.Retry)[i])
			if err != nil {
				return err
			}
		}
	}

	return nil
}
