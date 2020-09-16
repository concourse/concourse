package vars

import (
	"strings"
	"sync"
)

type TrackedVarsIterator interface {
	YieldCred(string, string)
}

type tracker struct {
	enabled bool

	// Considering in-parallel steps, a lock is need.
	lock              sync.RWMutex
	interpolatedCreds map[string]string
}

func newTracker(on bool) *tracker {
	return &tracker{
		enabled:           on,
		interpolatedCreds: map[string]string{},
	}
}

func (t *tracker) Track(varRef VariableReference, val interface{}) {
	if !t.enabled {
		return
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	t.track(varRef, val)
}

func (t *tracker) track(varRef VariableReference, val interface{}) {
	switch v := val.(type) {
	case map[interface{}]interface{}:
		for kk, vv := range v {
			t.track(VariableReference{
				Path:   varRef.Path,
				Fields: append(varRef.Fields, kk.(string)),
			}, vv)
		}
	case map[string]interface{}:
		for kk, vv := range v {
			t.track(VariableReference{
				Path:   varRef.Path,
				Fields: append(varRef.Fields, kk),
			}, vv)
		}
	case string:
		paths := append([]string{varRef.Path}, varRef.Fields...)

		t.interpolatedCreds[strings.Join(paths, ".")] = v
	default:
		// Do nothing
	}
}

func (t *tracker) IterateInterpolatedCreds(iter TrackedVarsIterator) {
	t.lock.RLock()
	for k, v := range t.interpolatedCreds {
		iter.YieldCred(k, v)
	}
	t.lock.RUnlock()
}

type credVarsTracker struct {
	*tracker
	credVars Variables
}

func (t *credVarsTracker) Get(varDef VariableDefinition) (interface{}, bool, error) {
	val, found, err := t.credVars.Get(varDef)
	if found {
		t.tracker.Track(varDef.Ref, val)
	}
	return val, found, err
}

func (t *credVarsTracker) List() ([]VariableDefinition, error) {
	return t.credVars.List()
}

// TrackedVarsMap is a TrackedVarsIterator which populates interpolated secrets into a map.
// If there are multiple secrets with the same name, it only keeps the first value.
type TrackedVarsMap map[string]string

func (it TrackedVarsMap) YieldCred(k, v string) {
	_, found := it[k]
	if !found {
		it[k] = v
	}
}
