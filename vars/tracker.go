package vars

import (
	"strings"
	"sync"
)

type TrackedVarsIterator interface {
	YieldCred(string, string)
}

type Tracker struct {
	Enabled bool

	// Considering in-parallel steps, a lock is need.
	lock              sync.RWMutex
	interpolatedCreds map[string]string
}

func NewTracker(on bool) *Tracker {
	return &Tracker{
		Enabled:           on,
		interpolatedCreds: map[string]string{},
	}
}

func (t *Tracker) Track(varRef Reference, val interface{}) {
	if !t.Enabled {
		return
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	t.track(varRef, val)
}

func (t *Tracker) track(varRef Reference, val interface{}) {
	switch v := val.(type) {
	case map[interface{}]interface{}:
		for kk, vv := range v {
			t.track(Reference{
				Path:   varRef.Path,
				Fields: append(varRef.Fields, kk.(string)),
			}, vv)
		}
	case map[string]interface{}:
		for kk, vv := range v {
			t.track(Reference{
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

func (t *Tracker) IterateInterpolatedCreds(iter TrackedVarsIterator) {
	t.lock.RLock()
	for k, v := range t.interpolatedCreds {
		iter.YieldCred(k, v)
	}
	t.lock.RUnlock()
}

type CredVarsTracker struct {
	*Tracker
	CredVars Variables
}

func (t *CredVarsTracker) Get(ref Reference) (interface{}, bool, error) {
	val, found, err := t.CredVars.Get(ref)
	if found {
		t.Tracker.Track(ref, val)
	}
	return val, found, err
}

func (t *CredVarsTracker) List() ([]Reference, error) {
	return t.CredVars.List()
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
