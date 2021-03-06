// Code generated by counterfeiter. DO NOT EDIT.
package metricfakes

import (
	"sync"

	"github.com/concourse/concourse/atc/metric"
)

type FakeEmitterFactory struct {
	DescriptionStub        func() string
	descriptionMutex       sync.RWMutex
	descriptionArgsForCall []struct {
	}
	descriptionReturns struct {
		result1 string
	}
	descriptionReturnsOnCall map[int]struct {
		result1 string
	}
	IsConfiguredStub        func() bool
	isConfiguredMutex       sync.RWMutex
	isConfiguredArgsForCall []struct {
	}
	isConfiguredReturns struct {
		result1 bool
	}
	isConfiguredReturnsOnCall map[int]struct {
		result1 bool
	}
	NewEmitterStub        func(map[string]string) (metric.Emitter, error)
	newEmitterMutex       sync.RWMutex
	newEmitterArgsForCall []struct {
		arg1 map[string]string
	}
	newEmitterReturns struct {
		result1 metric.Emitter
		result2 error
	}
	newEmitterReturnsOnCall map[int]struct {
		result1 metric.Emitter
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeEmitterFactory) Description() string {
	fake.descriptionMutex.Lock()
	ret, specificReturn := fake.descriptionReturnsOnCall[len(fake.descriptionArgsForCall)]
	fake.descriptionArgsForCall = append(fake.descriptionArgsForCall, struct {
	}{})
	stub := fake.DescriptionStub
	fakeReturns := fake.descriptionReturns
	fake.recordInvocation("Description", []interface{}{})
	fake.descriptionMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeEmitterFactory) DescriptionCallCount() int {
	fake.descriptionMutex.RLock()
	defer fake.descriptionMutex.RUnlock()
	return len(fake.descriptionArgsForCall)
}

func (fake *FakeEmitterFactory) DescriptionCalls(stub func() string) {
	fake.descriptionMutex.Lock()
	defer fake.descriptionMutex.Unlock()
	fake.DescriptionStub = stub
}

func (fake *FakeEmitterFactory) DescriptionReturns(result1 string) {
	fake.descriptionMutex.Lock()
	defer fake.descriptionMutex.Unlock()
	fake.DescriptionStub = nil
	fake.descriptionReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeEmitterFactory) DescriptionReturnsOnCall(i int, result1 string) {
	fake.descriptionMutex.Lock()
	defer fake.descriptionMutex.Unlock()
	fake.DescriptionStub = nil
	if fake.descriptionReturnsOnCall == nil {
		fake.descriptionReturnsOnCall = make(map[int]struct {
			result1 string
		})
	}
	fake.descriptionReturnsOnCall[i] = struct {
		result1 string
	}{result1}
}

func (fake *FakeEmitterFactory) IsConfigured() bool {
	fake.isConfiguredMutex.Lock()
	ret, specificReturn := fake.isConfiguredReturnsOnCall[len(fake.isConfiguredArgsForCall)]
	fake.isConfiguredArgsForCall = append(fake.isConfiguredArgsForCall, struct {
	}{})
	stub := fake.IsConfiguredStub
	fakeReturns := fake.isConfiguredReturns
	fake.recordInvocation("IsConfigured", []interface{}{})
	fake.isConfiguredMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeEmitterFactory) IsConfiguredCallCount() int {
	fake.isConfiguredMutex.RLock()
	defer fake.isConfiguredMutex.RUnlock()
	return len(fake.isConfiguredArgsForCall)
}

func (fake *FakeEmitterFactory) IsConfiguredCalls(stub func() bool) {
	fake.isConfiguredMutex.Lock()
	defer fake.isConfiguredMutex.Unlock()
	fake.IsConfiguredStub = stub
}

func (fake *FakeEmitterFactory) IsConfiguredReturns(result1 bool) {
	fake.isConfiguredMutex.Lock()
	defer fake.isConfiguredMutex.Unlock()
	fake.IsConfiguredStub = nil
	fake.isConfiguredReturns = struct {
		result1 bool
	}{result1}
}

func (fake *FakeEmitterFactory) IsConfiguredReturnsOnCall(i int, result1 bool) {
	fake.isConfiguredMutex.Lock()
	defer fake.isConfiguredMutex.Unlock()
	fake.IsConfiguredStub = nil
	if fake.isConfiguredReturnsOnCall == nil {
		fake.isConfiguredReturnsOnCall = make(map[int]struct {
			result1 bool
		})
	}
	fake.isConfiguredReturnsOnCall[i] = struct {
		result1 bool
	}{result1}
}

func (fake *FakeEmitterFactory) NewEmitter(arg1 map[string]string) (metric.Emitter, error) {
	fake.newEmitterMutex.Lock()
	ret, specificReturn := fake.newEmitterReturnsOnCall[len(fake.newEmitterArgsForCall)]
	fake.newEmitterArgsForCall = append(fake.newEmitterArgsForCall, struct {
		arg1 map[string]string
	}{arg1})
	stub := fake.NewEmitterStub
	fakeReturns := fake.newEmitterReturns
	fake.recordInvocation("NewEmitter", []interface{}{arg1})
	fake.newEmitterMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeEmitterFactory) NewEmitterCallCount() int {
	fake.newEmitterMutex.RLock()
	defer fake.newEmitterMutex.RUnlock()
	return len(fake.newEmitterArgsForCall)
}

func (fake *FakeEmitterFactory) NewEmitterCalls(stub func(map[string]string) (metric.Emitter, error)) {
	fake.newEmitterMutex.Lock()
	defer fake.newEmitterMutex.Unlock()
	fake.NewEmitterStub = stub
}

func (fake *FakeEmitterFactory) NewEmitterArgsForCall(i int) map[string]string {
	fake.newEmitterMutex.RLock()
	defer fake.newEmitterMutex.RUnlock()
	argsForCall := fake.newEmitterArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeEmitterFactory) NewEmitterReturns(result1 metric.Emitter, result2 error) {
	fake.newEmitterMutex.Lock()
	defer fake.newEmitterMutex.Unlock()
	fake.NewEmitterStub = nil
	fake.newEmitterReturns = struct {
		result1 metric.Emitter
		result2 error
	}{result1, result2}
}

func (fake *FakeEmitterFactory) NewEmitterReturnsOnCall(i int, result1 metric.Emitter, result2 error) {
	fake.newEmitterMutex.Lock()
	defer fake.newEmitterMutex.Unlock()
	fake.NewEmitterStub = nil
	if fake.newEmitterReturnsOnCall == nil {
		fake.newEmitterReturnsOnCall = make(map[int]struct {
			result1 metric.Emitter
			result2 error
		})
	}
	fake.newEmitterReturnsOnCall[i] = struct {
		result1 metric.Emitter
		result2 error
	}{result1, result2}
}

func (fake *FakeEmitterFactory) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.descriptionMutex.RLock()
	defer fake.descriptionMutex.RUnlock()
	fake.isConfiguredMutex.RLock()
	defer fake.isConfiguredMutex.RUnlock()
	fake.newEmitterMutex.RLock()
	defer fake.newEmitterMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeEmitterFactory) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ metric.EmitterFactory = new(FakeEmitterFactory)
