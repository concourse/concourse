package vars

import (
	"fmt"
	"sync"
)

// CredVarsTracker implements the interface Variables. It wraps a secret manager and
// tracks key-values fetched from the secret managers. It also provides a method to
// thread-safely iterate interpolated key-values.

type CredVarsTrackerIterator interface {
	YieldCred(string, string)
}

type CredVarsTracker interface {
	Variables
	IterateInterpolatedCreds(iter CredVarsTrackerIterator)
	Enabled() bool
}

func NewCredVarsTracker(credVars Variables, on bool) CredVarsTracker {
	if on {
		return &credVarsTracker{
			credVars:          credVars,
			interpolatedCreds: map[string]string{},
			lock:              sync.RWMutex{},
		}
	} else {
		return dummyCredVarsTracker{credVars: credVars}
	}
}

type credVarsTracker struct {
	credVars          Variables
	interpolatedCreds map[string]string

	// Considering in-parallel steps, a lock is need.
	lock sync.RWMutex
}

func (t *credVarsTracker) Get(varDef VariableDefinition) (interface{}, bool, error) {
	val, found, err := t.credVars.Get(varDef)
	if found {
		t.lock.Lock()
		t.track(varDef.Name, val)
		t.lock.Unlock()
	}

	return val, found, err
}

func (t *credVarsTracker) track(name string, val interface{}) {
	switch v := val.(type) {
	case map[interface{}]interface{}:
		for kk, vv := range v {
			nn := fmt.Sprintf("%s.%s", name, kk.(string))
			t.track(nn, vv)
		}
	case map[string]interface{}:
		for kk, vv := range v {
			nn := fmt.Sprintf("%s.%s", name, kk)
			t.track(nn, vv)
		}
	case string:
		t.interpolatedCreds[name] = v
	default:
		// Do nothing
	}
}

func (t *credVarsTracker) List() ([]VariableDefinition, error) {
	return t.credVars.List()
}

func (t *credVarsTracker) IterateInterpolatedCreds(iter CredVarsTrackerIterator) {
	t.lock.RLock()
	for k, v := range t.interpolatedCreds {
		iter.YieldCred(k, v)
	}
	t.lock.RUnlock()
}

func (t *credVarsTracker) Enabled() bool {
	return true
}

// DummyCredVarsTracker do nothing,

type dummyCredVarsTracker struct {
	credVars Variables
}

func (t dummyCredVarsTracker) Get(varDef VariableDefinition) (interface{}, bool, error) {
	return t.credVars.Get(varDef)
}

func (t dummyCredVarsTracker) List() ([]VariableDefinition, error) {
	return t.credVars.List()
}

func (t dummyCredVarsTracker) IterateInterpolatedCreds(iter CredVarsTrackerIterator) {
	// do nothing
}

func (t dummyCredVarsTracker) Enabled() bool {
	return false
}

// MapCredVarsTrackerIterator implements a simple CredVarsTrackerIterator which just
// populate interpolated secrets into a map. This could be useful in unit test.

type MapCredVarsTrackerIterator struct {
	Data map[string]interface{}
}

func NewMapCredVarsTrackerIterator() *MapCredVarsTrackerIterator {
	return &MapCredVarsTrackerIterator{
		Data: map[string]interface{}{},
	}
}

func (it *MapCredVarsTrackerIterator) YieldCred(k, v string) {
	it.Data[k] = v
}
