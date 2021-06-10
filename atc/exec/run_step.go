package exec

import (
	"context"
	"fmt"
	"io"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
)

// RunStep will run a message against a prototype.
type RunStep struct {
	planID          atc.PlanID
	plan            atc.RunPlan
	delegateFactory RunDelegateFactory
}

type RunDelegateFactory interface {
	RunDelegate(state RunState) RunDelegate
}

type RunDelegate interface {
	Stdout() io.Writer
	Stderr() io.Writer

	Initializing(lager.Logger)
	Starting(lager.Logger)
	Finished(lager.Logger, bool)
}

func NewRunStep(
	planID atc.PlanID,
	plan atc.RunPlan,
	delegateFactory RunDelegateFactory,
) Step {
	return &RunStep{
		planID:          planID,
		plan:            plan,
		delegateFactory: delegateFactory,
	}
}

func (step *RunStep) Run(ctx context.Context, state RunState) (bool, error) {
	logger := lagerctx.FromContext(ctx)

	delegate := step.delegateFactory.RunDelegate(state)
	delegate.Initializing(logger)

	stderr := delegate.Stderr()
	fmt.Fprint(stderr, "\x1b[1;33mthe run step is not yet implemented\x1b[0m\n\n")

	delegate.Starting(logger)

	fmt.Fprintf(stderr, "\x1b[1;34mpretending to run %s on prototype %s...\x1b[0m\n", step.plan.Message, step.plan.Type)

	delegate.Finished(logger, true)

	return true, nil
}
