// Code generated by counterfeiter. DO NOT EDIT.
package policycheckerfakes

import (
	"net/http"
	"sync"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/policychecker"
	"github.com/concourse/concourse/atc/policy"
)

type FakePolicyChecker struct {
	CheckStub        func(string, accessor.Access, *http.Request) (policy.PolicyCheckResult, error)
	checkMutex       sync.RWMutex
	checkArgsForCall []struct {
		arg1 string
		arg2 accessor.Access
		arg3 *http.Request
	}
	checkReturns struct {
		result1 policy.PolicyCheckResult
		result2 error
	}
	checkReturnsOnCall map[int]struct {
		result1 policy.PolicyCheckResult
		result2 error
	}
	invocations      map[string][][]any
	invocationsMutex sync.RWMutex
}

func (fake *FakePolicyChecker) Check(arg1 string, arg2 accessor.Access, arg3 *http.Request) (policy.PolicyCheckResult, error) {
	fake.checkMutex.Lock()
	ret, specificReturn := fake.checkReturnsOnCall[len(fake.checkArgsForCall)]
	fake.checkArgsForCall = append(fake.checkArgsForCall, struct {
		arg1 string
		arg2 accessor.Access
		arg3 *http.Request
	}{arg1, arg2, arg3})
	stub := fake.CheckStub
	fakeReturns := fake.checkReturns
	fake.recordInvocation("Check", []any{arg1, arg2, arg3})
	fake.checkMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakePolicyChecker) CheckCallCount() int {
	fake.checkMutex.RLock()
	defer fake.checkMutex.RUnlock()
	return len(fake.checkArgsForCall)
}

func (fake *FakePolicyChecker) CheckCalls(stub func(string, accessor.Access, *http.Request) (policy.PolicyCheckResult, error)) {
	fake.checkMutex.Lock()
	defer fake.checkMutex.Unlock()
	fake.CheckStub = stub
}

func (fake *FakePolicyChecker) CheckArgsForCall(i int) (string, accessor.Access, *http.Request) {
	fake.checkMutex.RLock()
	defer fake.checkMutex.RUnlock()
	argsForCall := fake.checkArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakePolicyChecker) CheckReturns(result1 policy.PolicyCheckResult, result2 error) {
	fake.checkMutex.Lock()
	defer fake.checkMutex.Unlock()
	fake.CheckStub = nil
	fake.checkReturns = struct {
		result1 policy.PolicyCheckResult
		result2 error
	}{result1, result2}
}

func (fake *FakePolicyChecker) CheckReturnsOnCall(i int, result1 policy.PolicyCheckResult, result2 error) {
	fake.checkMutex.Lock()
	defer fake.checkMutex.Unlock()
	fake.CheckStub = nil
	if fake.checkReturnsOnCall == nil {
		fake.checkReturnsOnCall = make(map[int]struct {
			result1 policy.PolicyCheckResult
			result2 error
		})
	}
	fake.checkReturnsOnCall[i] = struct {
		result1 policy.PolicyCheckResult
		result2 error
	}{result1, result2}
}

func (fake *FakePolicyChecker) Invocations() map[string][][]any {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.checkMutex.RLock()
	defer fake.checkMutex.RUnlock()
	copiedInvocations := map[string][][]any{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakePolicyChecker) recordInvocation(key string, args []any) {
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

var _ policychecker.PolicyChecker = new(FakePolicyChecker)
