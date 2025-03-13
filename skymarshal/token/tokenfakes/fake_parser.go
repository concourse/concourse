// Code generated by counterfeiter. DO NOT EDIT.
package tokenfakes

import (
	"sync"
	"time"

	"github.com/concourse/concourse/skymarshal/token"
)

type FakeParser struct {
	ParseExpiryStub        func(string) (time.Time, error)
	parseExpiryMutex       sync.RWMutex
	parseExpiryArgsForCall []struct {
		arg1 string
	}
	parseExpiryReturns struct {
		result1 time.Time
		result2 error
	}
	parseExpiryReturnsOnCall map[int]struct {
		result1 time.Time
		result2 error
	}
	invocations      map[string][][]any
	invocationsMutex sync.RWMutex
}

func (fake *FakeParser) ParseExpiry(arg1 string) (time.Time, error) {
	fake.parseExpiryMutex.Lock()
	ret, specificReturn := fake.parseExpiryReturnsOnCall[len(fake.parseExpiryArgsForCall)]
	fake.parseExpiryArgsForCall = append(fake.parseExpiryArgsForCall, struct {
		arg1 string
	}{arg1})
	stub := fake.ParseExpiryStub
	fakeReturns := fake.parseExpiryReturns
	fake.recordInvocation("ParseExpiry", []any{arg1})
	fake.parseExpiryMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeParser) ParseExpiryCallCount() int {
	fake.parseExpiryMutex.RLock()
	defer fake.parseExpiryMutex.RUnlock()
	return len(fake.parseExpiryArgsForCall)
}

func (fake *FakeParser) ParseExpiryCalls(stub func(string) (time.Time, error)) {
	fake.parseExpiryMutex.Lock()
	defer fake.parseExpiryMutex.Unlock()
	fake.ParseExpiryStub = stub
}

func (fake *FakeParser) ParseExpiryArgsForCall(i int) string {
	fake.parseExpiryMutex.RLock()
	defer fake.parseExpiryMutex.RUnlock()
	argsForCall := fake.parseExpiryArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeParser) ParseExpiryReturns(result1 time.Time, result2 error) {
	fake.parseExpiryMutex.Lock()
	defer fake.parseExpiryMutex.Unlock()
	fake.ParseExpiryStub = nil
	fake.parseExpiryReturns = struct {
		result1 time.Time
		result2 error
	}{result1, result2}
}

func (fake *FakeParser) ParseExpiryReturnsOnCall(i int, result1 time.Time, result2 error) {
	fake.parseExpiryMutex.Lock()
	defer fake.parseExpiryMutex.Unlock()
	fake.ParseExpiryStub = nil
	if fake.parseExpiryReturnsOnCall == nil {
		fake.parseExpiryReturnsOnCall = make(map[int]struct {
			result1 time.Time
			result2 error
		})
	}
	fake.parseExpiryReturnsOnCall[i] = struct {
		result1 time.Time
		result2 error
	}{result1, result2}
}

func (fake *FakeParser) Invocations() map[string][][]any {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.parseExpiryMutex.RLock()
	defer fake.parseExpiryMutex.RUnlock()
	copiedInvocations := map[string][][]any{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeParser) recordInvocation(key string, args []any) {
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

var _ token.Parser = new(FakeParser)
