package exec

import (
	"fmt"
	"os"
	"strings"

	"github.com/tedsuo/ifrit"
)

type Aggregate []StepFactory

func (a Aggregate) Using(prev Step, repo *SourceRepository) Step {
	sources := aggregateStep{}

	for _, step := range a {
		sources = append(sources, step.Using(prev, repo))
	}

	return sources
}

type aggregateStep []Step

func (step aggregateStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
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

dance:
	for _, mp := range members {
		select {
		case sig := <-signals:
			for _, mp := range members {
				mp.Signal(sig)
			}

			for _, mp := range members {
				err := <-mp.Wait()
				if err != nil {
					errorMessages = append(errorMessages, err.Error())
				}
			}

			break dance
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

func (source aggregateStep) Release() {

	for _, src := range source {
		src.Release()
	}
}

func (source aggregateStep) Result(x interface{}) bool {
	if success, ok := x.(*Success); ok {
		succeeded := true
		anyIndicated := false
		for _, src := range source {
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
