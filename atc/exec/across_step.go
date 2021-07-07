package exec

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/vars"
)

type AcrossStep struct {
	plan atc.AcrossPlan

	delegateFactory BuildStepDelegateFactory
	metadata        StepMetadata
}

// Across constructs an AcrossStep, which runs a substep for each combination
// of var values. These substeps are generated dynamically (and emitted as a
// build event) since the sets of var values may only be known at runtime (if
// they are interpolated).
//
// Substeps may be run in parallel, according to the max_in_flight
// configuration of the vars.
func Across(
	plan atc.AcrossPlan,
	delegateFactory BuildStepDelegateFactory,
	metadata StepMetadata,
) AcrossStep {
	return AcrossStep{
		plan:            plan,
		delegateFactory: delegateFactory,
		metadata:        metadata,
	}
}

func (step AcrossStep) Run(ctx context.Context, state RunState) (bool, error) {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("across-step", lager.Data{
		"job-id": step.metadata.JobID,
	})

	delegate := step.delegateFactory.BuildStepDelegate(state)

	delegate.Initializing(logger)

	stderr := delegate.Stderr()

	fmt.Fprintln(stderr, "\x1b[1;33mWARNING: the across step is experimental and subject to change!\x1b[0m")
	fmt.Fprintln(stderr, "")
	fmt.Fprintln(stderr, "\x1b[33mfollow RFC #29 for updates: https://github.com/concourse/rfcs/pull/29\x1b[0m")
	fmt.Fprintln(stderr, "")

	delegate.Starting(logger)

	varValues := make([][]interface{}, len(step.plan.Vars))
	for i, v := range step.plan.Vars {
		_, found, _ := state.Get(vars.Reference{Source: ".", Path: v.Var})
		if found {
			fmt.Fprintf(stderr, "\x1b[1;33mWARNING: across step shadows local var '%s'\x1b[0m\n", v.Var)
		}
		var err error
		varValues[i], err = creds.NewList(state, v.Values).Evaluate()
		if err != nil {
			return false, err
		}
	}
	substeps, err := delegate.ConstructAcrossSubsteps([]byte(step.plan.SubStepTemplate), step.plan.Vars, cartesianProduct(varValues))
	if err != nil {
		return false, err
	}

	exec := step.acrossStepExecutor(state, varValues, 0, substeps)
	succeeded, err := exec.run(ctx)
	if err != nil {
		return false, err
	}

	delegate.Finished(logger, succeeded)

	return succeeded, nil
}

func (step AcrossStep) acrossStepExecutor(state RunState, varValues [][]interface{}, varIndex int, steps []atc.VarScopedPlan) parallelExecutor {
	if varIndex == len(step.plan.Vars)-1 {
		return step.acrossStepLeafExecutor(state, steps)
	}
	stepsPerValue := 1
	for i := varIndex + 1; i < len(step.plan.Vars); i++ {
		stepsPerValue *= len(varValues[i])
	}
	numValues := len(varValues[varIndex])
	return parallelExecutor{
		stepName: "across",

		maxInFlight: step.plan.Vars[varIndex].MaxInFlight,
		failFast:    step.plan.FailFast,
		count:       numValues,

		runFunc: func(ctx context.Context, i int) (bool, error) {
			startIndex := i * stepsPerValue
			endIndex := (i + 1) * stepsPerValue
			substeps := steps[startIndex:endIndex]
			return step.acrossStepExecutor(state, varValues, varIndex+1, substeps).run(ctx)
		},
	}
}

func (step AcrossStep) acrossStepLeafExecutor(state RunState, steps []atc.VarScopedPlan) parallelExecutor {
	lastVar := step.plan.Vars[len(step.plan.Vars)-1]
	return parallelExecutor{
		stepName: "across",

		maxInFlight: lastVar.MaxInFlight,
		failFast:    step.plan.FailFast,
		count:       len(steps),

		runFunc: func(ctx context.Context, i int) (bool, error) {
			// Even though we interpolate the vars into the substep plan, we
			// still need to add them to a local scope since they can be used
			// to interpolate a task file.
			scope := state.NewLocalScope()
			for j, v := range step.plan.Vars {
				// Don't redact because the values being iterated across are
				// intended to be public - they're displayed directly in the UI
				scope.AddLocalVar(v.Var, steps[i].Values[j], false)
			}

			return scope.Run(ctx, steps[i].Step)
		},
	}
}

// cartesianProduct takes in a matrix of the values that each var takes on (in
// the same order as the Vars are defined on the plan), and returns the set of
// combinations of those variables. In particular:
//
// varValues[i][j] is the j'th value for variable i
// result[i][j] is the value of the j'th variable in combination i
//
// e.g. cartesianProduct([["a1", "a2"], ["b1"], ["c1", "c2"]])
//      = [
//          ["a1", "b1", "c1"],
//          ["a1", "b1", "c2"],
//          ["a2", "b1", "c1"],
//          ["a2", "b1", "c2"],
//        ]
func cartesianProduct(varValues [][]interface{}) [][]interface{} {
	if len(varValues) == 0 {
		return make([][]interface{}, 1)
	}
	var product [][]interface{}
	subProduct := cartesianProduct(varValues[:len(varValues)-1])
	for _, vec := range subProduct {
		for _, val := range varValues[len(varValues)-1] {
			product = append(product, append(vec, val))
		}
	}
	return product
}
