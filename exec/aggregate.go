package exec

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/tedsuo/ifrit"
)

type Aggregate map[string]StepFactory

func (a Aggregate) Using(prev Step, repo *SourceRepository) Step {
	sources := aggregateStep{}

	for name, step := range a {
		sources[name] = step.Using(prev, repo)
	}

	return sources
}

type aggregateStep map[string]Step

func (step aggregateStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	members := map[string]ifrit.Process{}

	for mn, ms := range step {
		process := ifrit.Background(ms)
		members[mn] = process
	}

	for _, mp := range members {
		select {
		case <-mp.Ready():
		case <-mp.Wait():
		}
	}

	close(ready)

	var errorMessages []string

	for mn, mp := range members {
		select {
		case sig := <-signals:
			for _, mp := range members {
				mp.Signal(sig)
			}

		case err := <-mp.Wait():
			if err != nil {
				errorMessages = append(errorMessages, mn+": "+err.Error())
			}
		}
	}

	if len(errorMessages) > 0 {
		return fmt.Errorf("sources failed:\n%s", strings.Join(errorMessages, "\n"))
	}

	return nil
}

func (source aggregateStep) Release() error {
	var errorMessages []string

	for name, src := range source {
		err := src.Release()
		if err != nil {
			errorMessages = append(errorMessages, name+": "+err.Error())
		}
	}

	if len(errorMessages) > 0 {
		return fmt.Errorf("sources failed to release:\n%s", strings.Join(errorMessages, "\n"))
	}

	return nil
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

	t := reflect.TypeOf(x)
	v := reflect.ValueOf(x)

	var m reflect.Value

	if t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Map {
		m = v.Elem()
	} else if t.Kind() == reflect.Map {
		m = v
	} else {
		return false
	}

	if m.Type().Key().Kind() != reflect.String {
		return false
	}

	for name, src := range source {
		res := reflect.New(m.Type().Elem())
		if !src.Result(res.Interface()) {
			return false
		}

		m.SetMapIndex(reflect.ValueOf(name), res.Elem())
	}

	return true
}
