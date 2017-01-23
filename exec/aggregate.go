package exec

import (
	"fmt"
	"os"
	"strings"

	"github.com/concourse/atc/worker"
	"github.com/tedsuo/ifrit"
)

// Aggregate constructs a Step that will run each step in parallel.
type Aggregate []StepFactory

// Using delegates to each StepFactory and returns an AggregateStep.
func (a Aggregate) Using(prev Step, repo *worker.ArtifactRepository) Step {
	sources := AggregateStep{}

	for _, step := range a {
		sources = append(sources, step.Using(prev, repo))
	}

	return sources
}

// AggregateStep is a step of steps to run in parallel.
type AggregateStep []Step

// Run executes all steps in parallel. It will indicate that it's ready when
// all of its steps are ready, and propagate any signal received to all running
// steps.
//
// It will wait for all steps to exit, even if one step fails or errors. After
// all steps finish, their errors (if any) will be aggregated and returned as a
// single error.
func (step AggregateStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	members := []ifrit.Process{}

	for _, ms := range step {
		process := ifrit.Background(ms)
		members = append(members, process)
	}

	for _, mp := range members {
		select {
		case <-mp.Ready():
		case <-mp.Wait():
		}
	}

	close(ready)

	var errorMessages []string

	for _, mp := range members {
		select {
		case sig := <-signals:
			for _, mp := range members {
				mp.Signal(sig)
			}

			for _, mp := range members {
				<-mp.Wait()
			}

			return ErrInterrupted
		case err := <-mp.Wait():
			if err != nil {
				errorMessages = append(errorMessages, err.Error())
			}
		}
	}

	if len(errorMessages) > 0 {
		return fmt.Errorf("sources failed:\n%s", strings.Join(errorMessages, "\n"))
	}

	return nil
}

// Result indicates Success as true if all of the steps indicate Success as
// true, or if there were no steps at all. If none of the steps can indicate
// Success, it will return false and not indicate success itself.
//
// All other result types are ignored, and Result will return false.
func (step AggregateStep) Result(x interface{}) bool {
	if success, ok := x.(*Success); ok {
		if len(step) == 0 {
			*success = Success(true)
			return true
		}

		succeeded := true
		anyIndicated := false
		for _, src := range step {
			var s Success
			if !src.Result(&s) {
				continue
			}

			anyIndicated = true
			succeeded = succeeded && bool(s)
		}

		if !anyIndicated {
			return false
		}

		*success = Success(succeeded)

		return true
	}

	return false
}
