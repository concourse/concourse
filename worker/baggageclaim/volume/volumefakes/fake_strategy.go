// Code generated by counterfeiter. DO NOT EDIT.
package volumefakes

import (
	"sync"

	lager "code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
)

type FakeStrategy struct {
	MaterializeStub        func(lager.Logger, string, volume.Filesystem, volume.Streamer) (volume.FilesystemInitVolume, error)
	materializeMutex       sync.RWMutex
	materializeArgsForCall []struct {
		arg1 lager.Logger
		arg2 string
		arg3 volume.Filesystem
		arg4 volume.Streamer
	}
	materializeReturns struct {
		result1 volume.FilesystemInitVolume
		result2 error
	}
	materializeReturnsOnCall map[int]struct {
		result1 volume.FilesystemInitVolume
		result2 error
	}
	StringStub        func() string
	stringMutex       sync.RWMutex
	stringArgsForCall []struct {
	}
	stringReturns struct {
		result1 string
	}
	stringReturnsOnCall map[int]struct {
		result1 string
	}
	invocations      map[string][][]any
	invocationsMutex sync.RWMutex
}

func (fake *FakeStrategy) Materialize(arg1 lager.Logger, arg2 string, arg3 volume.Filesystem, arg4 volume.Streamer) (volume.FilesystemInitVolume, error) {
	fake.materializeMutex.Lock()
	ret, specificReturn := fake.materializeReturnsOnCall[len(fake.materializeArgsForCall)]
	fake.materializeArgsForCall = append(fake.materializeArgsForCall, struct {
		arg1 lager.Logger
		arg2 string
		arg3 volume.Filesystem
		arg4 volume.Streamer
	}{arg1, arg2, arg3, arg4})
	stub := fake.MaterializeStub
	fakeReturns := fake.materializeReturns
	fake.recordInvocation("Materialize", []any{arg1, arg2, arg3, arg4})
	fake.materializeMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeStrategy) MaterializeCallCount() int {
	fake.materializeMutex.RLock()
	defer fake.materializeMutex.RUnlock()
	return len(fake.materializeArgsForCall)
}

func (fake *FakeStrategy) MaterializeCalls(stub func(lager.Logger, string, volume.Filesystem, volume.Streamer) (volume.FilesystemInitVolume, error)) {
	fake.materializeMutex.Lock()
	defer fake.materializeMutex.Unlock()
	fake.MaterializeStub = stub
}

func (fake *FakeStrategy) MaterializeArgsForCall(i int) (lager.Logger, string, volume.Filesystem, volume.Streamer) {
	fake.materializeMutex.RLock()
	defer fake.materializeMutex.RUnlock()
	argsForCall := fake.materializeArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeStrategy) MaterializeReturns(result1 volume.FilesystemInitVolume, result2 error) {
	fake.materializeMutex.Lock()
	defer fake.materializeMutex.Unlock()
	fake.MaterializeStub = nil
	fake.materializeReturns = struct {
		result1 volume.FilesystemInitVolume
		result2 error
	}{result1, result2}
}

func (fake *FakeStrategy) MaterializeReturnsOnCall(i int, result1 volume.FilesystemInitVolume, result2 error) {
	fake.materializeMutex.Lock()
	defer fake.materializeMutex.Unlock()
	fake.MaterializeStub = nil
	if fake.materializeReturnsOnCall == nil {
		fake.materializeReturnsOnCall = make(map[int]struct {
			result1 volume.FilesystemInitVolume
			result2 error
		})
	}
	fake.materializeReturnsOnCall[i] = struct {
		result1 volume.FilesystemInitVolume
		result2 error
	}{result1, result2}
}

func (fake *FakeStrategy) String() string {
	fake.stringMutex.Lock()
	ret, specificReturn := fake.stringReturnsOnCall[len(fake.stringArgsForCall)]
	fake.stringArgsForCall = append(fake.stringArgsForCall, struct {
	}{})
	stub := fake.StringStub
	fakeReturns := fake.stringReturns
	fake.recordInvocation("String", []any{})
	fake.stringMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeStrategy) StringCallCount() int {
	fake.stringMutex.RLock()
	defer fake.stringMutex.RUnlock()
	return len(fake.stringArgsForCall)
}

func (fake *FakeStrategy) StringCalls(stub func() string) {
	fake.stringMutex.Lock()
	defer fake.stringMutex.Unlock()
	fake.StringStub = stub
}

func (fake *FakeStrategy) StringReturns(result1 string) {
	fake.stringMutex.Lock()
	defer fake.stringMutex.Unlock()
	fake.StringStub = nil
	fake.stringReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeStrategy) StringReturnsOnCall(i int, result1 string) {
	fake.stringMutex.Lock()
	defer fake.stringMutex.Unlock()
	fake.StringStub = nil
	if fake.stringReturnsOnCall == nil {
		fake.stringReturnsOnCall = make(map[int]struct {
			result1 string
		})
	}
	fake.stringReturnsOnCall[i] = struct {
		result1 string
	}{result1}
}

func (fake *FakeStrategy) Invocations() map[string][][]any {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.materializeMutex.RLock()
	defer fake.materializeMutex.RUnlock()
	fake.stringMutex.RLock()
	defer fake.stringMutex.RUnlock()
	copiedInvocations := map[string][][]any{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeStrategy) recordInvocation(key string, args []any) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]any{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]any{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ volume.Strategy = new(FakeStrategy)
