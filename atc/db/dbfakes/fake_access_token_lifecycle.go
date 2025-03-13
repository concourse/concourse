// Code generated by counterfeiter. DO NOT EDIT.
package dbfakes

import (
	"sync"
	"time"

	"github.com/concourse/concourse/atc/db"
)

type FakeAccessTokenLifecycle struct {
	RemoveExpiredAccessTokensStub        func(time.Duration) (int, error)
	removeExpiredAccessTokensMutex       sync.RWMutex
	removeExpiredAccessTokensArgsForCall []struct {
		arg1 time.Duration
	}
	removeExpiredAccessTokensReturns struct {
		result1 int
		result2 error
	}
	removeExpiredAccessTokensReturnsOnCall map[int]struct {
		result1 int
		result2 error
	}
	invocations      map[string][][]any
	invocationsMutex sync.RWMutex
}

func (fake *FakeAccessTokenLifecycle) RemoveExpiredAccessTokens(arg1 time.Duration) (int, error) {
	fake.removeExpiredAccessTokensMutex.Lock()
	ret, specificReturn := fake.removeExpiredAccessTokensReturnsOnCall[len(fake.removeExpiredAccessTokensArgsForCall)]
	fake.removeExpiredAccessTokensArgsForCall = append(fake.removeExpiredAccessTokensArgsForCall, struct {
		arg1 time.Duration
	}{arg1})
	stub := fake.RemoveExpiredAccessTokensStub
	fakeReturns := fake.removeExpiredAccessTokensReturns
	fake.recordInvocation("RemoveExpiredAccessTokens", []any{arg1})
	fake.removeExpiredAccessTokensMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeAccessTokenLifecycle) RemoveExpiredAccessTokensCallCount() int {
	fake.removeExpiredAccessTokensMutex.RLock()
	defer fake.removeExpiredAccessTokensMutex.RUnlock()
	return len(fake.removeExpiredAccessTokensArgsForCall)
}

func (fake *FakeAccessTokenLifecycle) RemoveExpiredAccessTokensCalls(stub func(time.Duration) (int, error)) {
	fake.removeExpiredAccessTokensMutex.Lock()
	defer fake.removeExpiredAccessTokensMutex.Unlock()
	fake.RemoveExpiredAccessTokensStub = stub
}

func (fake *FakeAccessTokenLifecycle) RemoveExpiredAccessTokensArgsForCall(i int) time.Duration {
	fake.removeExpiredAccessTokensMutex.RLock()
	defer fake.removeExpiredAccessTokensMutex.RUnlock()
	argsForCall := fake.removeExpiredAccessTokensArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeAccessTokenLifecycle) RemoveExpiredAccessTokensReturns(result1 int, result2 error) {
	fake.removeExpiredAccessTokensMutex.Lock()
	defer fake.removeExpiredAccessTokensMutex.Unlock()
	fake.RemoveExpiredAccessTokensStub = nil
	fake.removeExpiredAccessTokensReturns = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *FakeAccessTokenLifecycle) RemoveExpiredAccessTokensReturnsOnCall(i int, result1 int, result2 error) {
	fake.removeExpiredAccessTokensMutex.Lock()
	defer fake.removeExpiredAccessTokensMutex.Unlock()
	fake.RemoveExpiredAccessTokensStub = nil
	if fake.removeExpiredAccessTokensReturnsOnCall == nil {
		fake.removeExpiredAccessTokensReturnsOnCall = make(map[int]struct {
			result1 int
			result2 error
		})
	}
	fake.removeExpiredAccessTokensReturnsOnCall[i] = struct {
		result1 int
		result2 error
	}{result1, result2}
}

func (fake *FakeAccessTokenLifecycle) Invocations() map[string][][]any {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.removeExpiredAccessTokensMutex.RLock()
	defer fake.removeExpiredAccessTokensMutex.RUnlock()
	copiedInvocations := map[string][][]any{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeAccessTokenLifecycle) recordInvocation(key string, args []any) {
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

var _ db.AccessTokenLifecycle = new(FakeAccessTokenLifecycle)
