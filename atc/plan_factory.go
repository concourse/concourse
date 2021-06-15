package atc

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
)

type PlanFactory struct {
	currentNum *int64
}

func NewPlanFactory(startingNum int64) PlanFactory {
	return PlanFactory{
		currentNum: &startingNum,
	}
}

type PlanConfig interface {
	Public() *json.RawMessage
}

func (factory PlanFactory) NewPlan(step PlanConfig) Plan {
	num := atomic.AddInt64(factory.currentNum, 1)

	var plan Plan
	switch t := step.(type) {
	case InParallelPlan:
		plan.InParallel = &t
	case AcrossPlan:
		plan.Across = &t
	case DoPlan:
		plan.Do = &t
	case GetPlan:
		plan.Get = &t
	case PutPlan:
		plan.Put = &t
	case TaskPlan:
		plan.Task = &t
	case RunPlan:
		plan.Run = &t
	case SetPipelinePlan:
		plan.SetPipeline = &t
	case LoadVarPlan:
		plan.LoadVar = &t
	case CheckPlan:
		plan.Check = &t
	case OnAbortPlan:
		plan.OnAbort = &t
	case OnErrorPlan:
		plan.OnError = &t
	case EnsurePlan:
		plan.Ensure = &t
	case OnSuccessPlan:
		plan.OnSuccess = &t
	case OnFailurePlan:
		plan.OnFailure = &t
	case TryPlan:
		plan.Try = &t
	case TimeoutPlan:
		plan.Timeout = &t
	case RetryPlan:
		plan.Retry = &t
	case ArtifactInputPlan:
		plan.ArtifactInput = &t
	case ArtifactOutputPlan:
		plan.ArtifactOutput = &t
	default:
		panic(fmt.Sprintf("don't know how to construct plan from %T", step))
	}

	plan.ID = PlanID(fmt.Sprintf("%x", num))

	return plan
}
