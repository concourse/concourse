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
func (a Aggregate) Using(repo *worker.ArtifactRepository) Step {
	sources := AggregateStep{}

	for _, step := range a {
		sources = append(sources, step.Using(repo))
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

// Succeeded is true if all of the steps' Succeeded is true
func (step AggregateStep) Succeeded() bool {
	succeeded := true

	for _, src := range step {
		if !src.Succeeded() {
			succeeded = false
		}
	}

	return succeeded
}
