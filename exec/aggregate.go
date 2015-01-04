package exec

import (
	"fmt"
	"io"
	"os"
	"path"
	"reflect"
	"strings"

	"github.com/tedsuo/ifrit"
)

type Aggregate map[string]Step

func (a Aggregate) Using(source ArtifactSource) ArtifactSource {
	sources := aggregateSource{}

	for name, step := range a {
		sources[name] = step.Using(source)
	}

	return sources
}

type aggregateSource map[string]ArtifactSource

func (source aggregateSource) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	members := map[string]ifrit.Process{}

	for mn, ms := range source {
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

func (source aggregateSource) StreamTo(dest ArtifactDestination) error {
	for name, src := range source {
		err := src.StreamTo(subdirectoryDestination{dest, name})
		if err != nil {
			return err
		}
	}

	return nil
}

func (source aggregateSource) StreamFile(path string) (io.ReadCloser, error) {
	for name, src := range source {
		if strings.HasPrefix(path, name+"/") {
			return src.StreamFile(path[len(name)+1:])
		}
	}

	return nil, ErrFileNotFound
}

func (source aggregateSource) Release() error {
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

func (source aggregateSource) Result(x interface{}) bool {
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

type subdirectoryDestination struct {
	destination  ArtifactDestination
	subdirectory string
}

func (dest subdirectoryDestination) StreamIn(destPath string, src io.Reader) error {
	return dest.destination.StreamIn(path.Join(dest.subdirectory, destPath), src)
}
