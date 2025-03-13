// Code generated by counterfeiter. DO NOT EDIT.
package enginefakes

import (
	"sync"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/exec"
)

type FakeStepperFactory struct {
	StepperForBuildStub        func(db.Build) (exec.Stepper, error)
	stepperForBuildMutex       sync.RWMutex
	stepperForBuildArgsForCall []struct {
		arg1 db.Build
	}
	stepperForBuildReturns struct {
		result1 exec.Stepper
		result2 error
	}
	stepperForBuildReturnsOnCall map[int]struct {
		result1 exec.Stepper
		result2 error
	}
	invocations      map[string][][]any
	invocationsMutex sync.RWMutex
}

func (fake *FakeStepperFactory) StepperForBuild(arg1 db.Build) (exec.Stepper, error) {
	fake.stepperForBuildMutex.Lock()
	ret, specificReturn := fake.stepperForBuildReturnsOnCall[len(fake.stepperForBuildArgsForCall)]
	fake.stepperForBuildArgsForCall = append(fake.stepperForBuildArgsForCall, struct {
		arg1 db.Build
	}{arg1})
	stub := fake.StepperForBuildStub
	fakeReturns := fake.stepperForBuildReturns
	fake.recordInvocation("StepperForBuild", []any{arg1})
	fake.stepperForBuildMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeStepperFactory) StepperForBuildCallCount() int {
	fake.stepperForBuildMutex.RLock()
	defer fake.stepperForBuildMutex.RUnlock()
	return len(fake.stepperForBuildArgsForCall)
}

func (fake *FakeStepperFactory) StepperForBuildCalls(stub func(db.Build) (exec.Stepper, error)) {
	fake.stepperForBuildMutex.Lock()
	defer fake.stepperForBuildMutex.Unlock()
	fake.StepperForBuildStub = stub
}

func (fake *FakeStepperFactory) StepperForBuildArgsForCall(i int) db.Build {
	fake.stepperForBuildMutex.RLock()
	defer fake.stepperForBuildMutex.RUnlock()
	argsForCall := fake.stepperForBuildArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeStepperFactory) StepperForBuildReturns(result1 exec.Stepper, result2 error) {
	fake.stepperForBuildMutex.Lock()
	defer fake.stepperForBuildMutex.Unlock()
	fake.StepperForBuildStub = nil
	fake.stepperForBuildReturns = struct {
		result1 exec.Stepper
		result2 error
	}{result1, result2}
}

func (fake *FakeStepperFactory) StepperForBuildReturnsOnCall(i int, result1 exec.Stepper, result2 error) {
	fake.stepperForBuildMutex.Lock()
	defer fake.stepperForBuildMutex.Unlock()
	fake.StepperForBuildStub = nil
	if fake.stepperForBuildReturnsOnCall == nil {
		fake.stepperForBuildReturnsOnCall = make(map[int]struct {
			result1 exec.Stepper
			result2 error
		})
	}
	fake.stepperForBuildReturnsOnCall[i] = struct {
		result1 exec.Stepper
		result2 error
	}{result1, result2}
}

func (fake *FakeStepperFactory) Invocations() map[string][][]any {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.stepperForBuildMutex.RLock()
	defer fake.stepperForBuildMutex.RUnlock()
	copiedInvocations := map[string][][]any{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeStepperFactory) recordInvocation(key string, args []any) {
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

var _ engine.StepperFactory = new(FakeStepperFactory)
