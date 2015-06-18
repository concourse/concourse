package exec

import (
	"os"

	"github.com/concourse/atc"
)

type Conditional struct {
	Conditions  atc.Conditions
	StepFactory StepFactory

	prev Step
	repo *SourceRepository

	result Step
}

func (c Conditional) Using(prev Step, repo *SourceRepository) Step {
	c.prev = prev
	c.repo = repo
	return &c
}

func (c *Conditional) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	var succeeded Success

	conditionMatched := false
	if c.prev.Result(&succeeded) {
		conditionMatched = c.Conditions.SatisfiedBy(bool(succeeded))
	}

	if conditionMatched {
		c.result = c.StepFactory.Using(c.prev, c.repo)
	} else {
		c.result = &NoopStep{}
	}

	return c.result.Run(signals, ready)
}

func (c *Conditional) Release() error {
	return c.result.Release()
}

func (c *Conditional) Result(x interface{}) bool {
	return c.result.Result(x)
}
