package vars

import (
	"strings"
	"sync"
)

// CredVarsTracker implements the interface Variables. It wraps a secret manager and
// tracks key-values fetched from the secret managers. It also provides a method to
// thread-safely iterate interpolated key-values.
type CredVarsTrackerIterator interface {
	YieldCred(string, string)
}

//go:generate counterfeiter . CredVarsTracker

type CredVarsTracker interface {
	Variables
	IterateInterpolatedCreds(iter CredVarsTrackerIterator)
	Enabled() bool

	AddLocalVar(string, interface{}, bool)
}

func NewCredVarsTracker(credVars Variables, on bool) CredVarsTracker {
	return &credVarsTracker{
		localVars:         StaticVariables{},
		credVars:          credVars,
		enabled:           on,
		interpolatedCreds: map[string]string{},
		noRedactVarNames:  map[string]bool{},
		lock:              sync.RWMutex{},
	}
}

type credVarsTracker struct {
	credVars  Variables
	localVars StaticVariables

	enabled bool

	interpolatedCreds map[string]string

	noRedactVarNames map[string]bool

	// Considering in-parallel steps, a lock is need.
	lock sync.RWMutex
}

func (t *credVarsTracker) Get(varDef VariableDefinition) (interface{}, bool, error) {
	var (
		val   interface{}
		found bool
		err   error
	)

	redact := true
	if varDef.Ref.Source == "." {
		val, found, err = t.localVars.Get(varDef)
		if found {
			if _, ok := t.noRedactVarNames[varDef.Ref.Path]; ok {
				redact = false
			}
		}
	} else {
		val, found, err = t.credVars.Get(varDef)
	}

	if t.enabled && found && redact {
		t.lock.Lock()
		t.track(varDef.Ref, val)
		t.lock.Unlock()
	}

	return val, found, err
}

func (t *credVarsTracker) track(varRef VariableReference, val interface{}) {
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
	// switch v := val.(type) {
	// case map[interface{}]interface{}:
	// 	var found bool
	// 	val, found = v[field]
	// 	if found {
	// 		varPath = fmt.Sprintf("%s.%s", varPath, field)
	// 	}
	// case map[string]interface{}:
	// 	var found bool
	// 	val, found = v[field]
	// 	if found {
	// 		varPath = fmt.Sprintf("%s.%s", varPath, field)
	// 	}
	// case string:
	// 	t.interpolatedCreds[varPath] = v
	// default:
	// 	// Do nothing
	// }

	// case map[interface{}]interface{}:
	// 	for kk, vv := range v {
	// 		nn := fmt.Sprintf("%s.%s", name, kk.(string))
	// 		t.track(nn, vv)
	// 	}
	// case map[string]interface{}:
	// 	for kk, vv := range v {
	// 		nn := fmt.Sprintf("%s.%s", name, kk)
	// 		t.track(nn, vv)
	// 	}
	// case string:
	// 	t.interpolatedCreds[name] = v
	// default:
	// 	// Do nothing
	// }
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
	return t.enabled
}

func (t *credVarsTracker) AddLocalVar(name string, value interface{}, redact bool) {
	t.localVars[name] = value
	if !redact {
		t.noRedactVarNames[name] = true
	}
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
