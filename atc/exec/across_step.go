package exec

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/vars"
)

type ScopedStep struct {
	Step
	Values []interface{}
}

// AcrossStep is a step of steps to run in parallel. It behaves the same as InParallelStep
// with the exception that an experimental warning is logged to stderr and that step
// lifecycle build events are emitted (Initializing, Starting, and Finished)
type AcrossStep struct {
	vars     []atc.AcrossVar
	steps    []ScopedStep
	failFast bool

	delegateFactory BuildStepDelegateFactory
	metadata        StepMetadata
}

// Across constructs an AcrossStep.
func Across(
	vars []atc.AcrossVar,
	steps []ScopedStep,
	failFast bool,
	delegateFactory BuildStepDelegateFactory,
	metadata StepMetadata,
) AcrossStep {
	return AcrossStep{
		vars:            vars,
		steps:           steps,
		failFast:        failFast,
		delegateFactory: delegateFactory,
		metadata:        metadata,
	}
}

// Run calls out to InParallelStep.Run after logging a warning to stderr. It also emits
// step lifecycle build events (Initializing, Starting, and Finished).
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

	for _, v := range step.vars {
		_, found, _ := state.Get(vars.Reference{Source: ".", Path: v.Var})
		if found {
			fmt.Fprintf(stderr, "\x1b[1;33mWARNING: across step shadows local var '%s'\x1b[0m\n", v.Var)
		}
	}

	delegate.Starting(logger)

	exec := step.acrossStepExecutor(state, 0, step.steps)
	succeeded, err := exec.run(ctx)
	if err != nil {
		return false, err
	}

	delegate.Finished(logger, succeeded)

	return succeeded, nil
}

func (step AcrossStep) acrossStepExecutor(state RunState, varIndex int, steps []ScopedStep) parallelExecutor {
	if varIndex == len(step.vars)-1 {
		return step.acrossStepLeafExecutor(state, steps)
	}
	stepsPerValue := 1
	for _, v := range step.vars[varIndex+1:] {
		stepsPerValue *= len(v.Values)
	}
	numValues := len(step.vars[varIndex].Values)
	return parallelExecutor{
		stepName: "across",

		maxInFlight: step.vars[varIndex].MaxInFlight,
		failFast:    step.failFast,
		count:       numValues,

		runFunc: func(ctx context.Context, i int) (bool, error) {
			startIndex := i * stepsPerValue
			endIndex := (i + 1) * stepsPerValue
			substeps := steps[startIndex:endIndex]
			return step.acrossStepExecutor(state, varIndex+1, substeps).run(ctx)
		},
	}
}

func (step AcrossStep) acrossStepLeafExecutor(state RunState, steps []ScopedStep) parallelExecutor {
	lastVar := step.vars[len(step.vars)-1]
	return parallelExecutor{
		stepName: "across",

		maxInFlight: lastVar.MaxInFlight,
		failFast:    step.failFast,
		count:       len(steps),

		runFunc: func(ctx context.Context, i int) (bool, error) {
			scope := state.NewLocalScope()
			for j, v := range step.vars {
				// Don't redact because the `list` operation of a var_source should return identifiers
				// which should be publicly accessible. For static across steps, the static list is
				// embedded directly in the pipeline
				scope.AddLocalVar(v.Var, steps[i].Values[j], false)
			}

			return steps[i].Run(ctx, scope)
		},
	}
}
