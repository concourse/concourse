// Code generated by counterfeiter. DO NOT EDIT.
package transportfakes

import (
	"sync"

	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/worker/gardenruntime/transport"
)

type FakeTransportDB struct {
	GetWorkerStub        func(string) (db.Worker, bool, error)
	getWorkerMutex       sync.RWMutex
	getWorkerArgsForCall []struct {
		arg1 string
	}
	getWorkerReturns struct {
		result1 db.Worker
		result2 bool
		result3 error
	}
	getWorkerReturnsOnCall map[int]struct {
		result1 db.Worker
		result2 bool
		result3 error
	}
	invocations      map[string][][]any
	invocationsMutex sync.RWMutex
}

func (fake *FakeTransportDB) GetWorker(arg1 string) (db.Worker, bool, error) {
	fake.getWorkerMutex.Lock()
	ret, specificReturn := fake.getWorkerReturnsOnCall[len(fake.getWorkerArgsForCall)]
	fake.getWorkerArgsForCall = append(fake.getWorkerArgsForCall, struct {
		arg1 string
	}{arg1})
	stub := fake.GetWorkerStub
	fakeReturns := fake.getWorkerReturns
	fake.recordInvocation("GetWorker", []any{arg1})
	fake.getWorkerMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2, ret.result3
	}
	return fakeReturns.result1, fakeReturns.result2, fakeReturns.result3
}

func (fake *FakeTransportDB) GetWorkerCallCount() int {
	fake.getWorkerMutex.RLock()
	defer fake.getWorkerMutex.RUnlock()
	return len(fake.getWorkerArgsForCall)
}

func (fake *FakeTransportDB) GetWorkerCalls(stub func(string) (db.Worker, bool, error)) {
	fake.getWorkerMutex.Lock()
	defer fake.getWorkerMutex.Unlock()
	fake.GetWorkerStub = stub
}

func (fake *FakeTransportDB) GetWorkerArgsForCall(i int) string {
	fake.getWorkerMutex.RLock()
	defer fake.getWorkerMutex.RUnlock()
	argsForCall := fake.getWorkerArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeTransportDB) GetWorkerReturns(result1 db.Worker, result2 bool, result3 error) {
	fake.getWorkerMutex.Lock()
	defer fake.getWorkerMutex.Unlock()
	fake.GetWorkerStub = nil
	fake.getWorkerReturns = struct {
		result1 db.Worker
		result2 bool
		result3 error
	}{result1, result2, result3}
}

func (fake *FakeTransportDB) GetWorkerReturnsOnCall(i int, result1 db.Worker, result2 bool, result3 error) {
	fake.getWorkerMutex.Lock()
	defer fake.getWorkerMutex.Unlock()
	fake.GetWorkerStub = nil
	if fake.getWorkerReturnsOnCall == nil {
		fake.getWorkerReturnsOnCall = make(map[int]struct {
			result1 db.Worker
			result2 bool
			result3 error
		})
	}
	fake.getWorkerReturnsOnCall[i] = struct {
		result1 db.Worker
		result2 bool
		result3 error
	}{result1, result2, result3}
}

func (fake *FakeTransportDB) Invocations() map[string][][]any {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.getWorkerMutex.RLock()
	defer fake.getWorkerMutex.RUnlock()
	copiedInvocations := map[string][][]any{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeTransportDB) recordInvocation(key string, args []any) {
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

var _ transport.TransportDB = new(FakeTransportDB)
