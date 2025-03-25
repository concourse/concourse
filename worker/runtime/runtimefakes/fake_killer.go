// Code generated by counterfeiter. DO NOT EDIT.
package runtimefakes

import (
	"context"
	"sync"

	"github.com/concourse/concourse/worker/runtime"
	"github.com/containerd/containerd/v2/client"
)

type FakeKiller struct {
	KillStub        func(context.Context, client.Task, runtime.KillBehaviour) error
	killMutex       sync.RWMutex
	killArgsForCall []struct {
		arg1 context.Context
		arg2 client.Task
		arg3 runtime.KillBehaviour
	}
	killReturns struct {
		result1 error
	}
	killReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeKiller) Kill(arg1 context.Context, arg2 client.Task, arg3 runtime.KillBehaviour) error {
	fake.killMutex.Lock()
	ret, specificReturn := fake.killReturnsOnCall[len(fake.killArgsForCall)]
	fake.killArgsForCall = append(fake.killArgsForCall, struct {
		arg1 context.Context
		arg2 client.Task
		arg3 runtime.KillBehaviour
	}{arg1, arg2, arg3})
	stub := fake.KillStub
	fakeReturns := fake.killReturns
	fake.recordInvocation("Kill", []interface{}{arg1, arg2, arg3})
	fake.killMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeKiller) KillCallCount() int {
	fake.killMutex.RLock()
	defer fake.killMutex.RUnlock()
	return len(fake.killArgsForCall)
}

func (fake *FakeKiller) KillCalls(stub func(context.Context, client.Task, runtime.KillBehaviour) error) {
	fake.killMutex.Lock()
	defer fake.killMutex.Unlock()
	fake.KillStub = stub
}

func (fake *FakeKiller) KillArgsForCall(i int) (context.Context, client.Task, runtime.KillBehaviour) {
	fake.killMutex.RLock()
	defer fake.killMutex.RUnlock()
	argsForCall := fake.killArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeKiller) KillReturns(result1 error) {
	fake.killMutex.Lock()
	defer fake.killMutex.Unlock()
	fake.KillStub = nil
	fake.killReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeKiller) KillReturnsOnCall(i int, result1 error) {
	fake.killMutex.Lock()
	defer fake.killMutex.Unlock()
	fake.KillStub = nil
	if fake.killReturnsOnCall == nil {
		fake.killReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.killReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeKiller) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.killMutex.RLock()
	defer fake.killMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeKiller) recordInvocation(key string, args []interface{}) {
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

var _ runtime.Killer = new(FakeKiller)
