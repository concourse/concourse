// Code generated by counterfeiter. DO NOT EDIT.
package dbfakes

import (
	"sync"

	"github.com/concourse/concourse/atc/db"
)

type FakeCreatingContainer struct {
	CreatedStub        func() (db.CreatedContainer, error)
	createdMutex       sync.RWMutex
	createdArgsForCall []struct {
	}
	createdReturns struct {
		result1 db.CreatedContainer
		result2 error
	}
	createdReturnsOnCall map[int]struct {
		result1 db.CreatedContainer
		result2 error
	}
	FailedStub        func() (db.FailedContainer, error)
	failedMutex       sync.RWMutex
	failedArgsForCall []struct {
	}
	failedReturns struct {
		result1 db.FailedContainer
		result2 error
	}
	failedReturnsOnCall map[int]struct {
		result1 db.FailedContainer
		result2 error
	}
	HandleStub        func() string
	handleMutex       sync.RWMutex
	handleArgsForCall []struct {
	}
	handleReturns struct {
		result1 string
	}
	handleReturnsOnCall map[int]struct {
		result1 string
	}
	IDStub        func() int
	iDMutex       sync.RWMutex
	iDArgsForCall []struct {
	}
	iDReturns struct {
		result1 int
	}
	iDReturnsOnCall map[int]struct {
		result1 int
	}
	MetadataStub        func() db.ContainerMetadata
	metadataMutex       sync.RWMutex
	metadataArgsForCall []struct {
	}
	metadataReturns struct {
		result1 db.ContainerMetadata
	}
	metadataReturnsOnCall map[int]struct {
		result1 db.ContainerMetadata
	}
	StateStub        func() string
	stateMutex       sync.RWMutex
	stateArgsForCall []struct {
	}
	stateReturns struct {
		result1 string
	}
	stateReturnsOnCall map[int]struct {
		result1 string
	}
	WorkerNameStub        func() string
	workerNameMutex       sync.RWMutex
	workerNameArgsForCall []struct {
	}
	workerNameReturns struct {
		result1 string
	}
	workerNameReturnsOnCall map[int]struct {
		result1 string
	}
	invocations      map[string][][]any
	invocationsMutex sync.RWMutex
}

func (fake *FakeCreatingContainer) Created() (db.CreatedContainer, error) {
	fake.createdMutex.Lock()
	ret, specificReturn := fake.createdReturnsOnCall[len(fake.createdArgsForCall)]
	fake.createdArgsForCall = append(fake.createdArgsForCall, struct {
	}{})
	stub := fake.CreatedStub
	fakeReturns := fake.createdReturns
	fake.recordInvocation("Created", []any{})
	fake.createdMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeCreatingContainer) CreatedCallCount() int {
	fake.createdMutex.RLock()
	defer fake.createdMutex.RUnlock()
	return len(fake.createdArgsForCall)
}

func (fake *FakeCreatingContainer) CreatedCalls(stub func() (db.CreatedContainer, error)) {
	fake.createdMutex.Lock()
	defer fake.createdMutex.Unlock()
	fake.CreatedStub = stub
}

func (fake *FakeCreatingContainer) CreatedReturns(result1 db.CreatedContainer, result2 error) {
	fake.createdMutex.Lock()
	defer fake.createdMutex.Unlock()
	fake.CreatedStub = nil
	fake.createdReturns = struct {
		result1 db.CreatedContainer
		result2 error
	}{result1, result2}
}

func (fake *FakeCreatingContainer) CreatedReturnsOnCall(i int, result1 db.CreatedContainer, result2 error) {
	fake.createdMutex.Lock()
	defer fake.createdMutex.Unlock()
	fake.CreatedStub = nil
	if fake.createdReturnsOnCall == nil {
		fake.createdReturnsOnCall = make(map[int]struct {
			result1 db.CreatedContainer
			result2 error
		})
	}
	fake.createdReturnsOnCall[i] = struct {
		result1 db.CreatedContainer
		result2 error
	}{result1, result2}
}

func (fake *FakeCreatingContainer) Failed() (db.FailedContainer, error) {
	fake.failedMutex.Lock()
	ret, specificReturn := fake.failedReturnsOnCall[len(fake.failedArgsForCall)]
	fake.failedArgsForCall = append(fake.failedArgsForCall, struct {
	}{})
	stub := fake.FailedStub
	fakeReturns := fake.failedReturns
	fake.recordInvocation("Failed", []any{})
	fake.failedMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeCreatingContainer) FailedCallCount() int {
	fake.failedMutex.RLock()
	defer fake.failedMutex.RUnlock()
	return len(fake.failedArgsForCall)
}

func (fake *FakeCreatingContainer) FailedCalls(stub func() (db.FailedContainer, error)) {
	fake.failedMutex.Lock()
	defer fake.failedMutex.Unlock()
	fake.FailedStub = stub
}

func (fake *FakeCreatingContainer) FailedReturns(result1 db.FailedContainer, result2 error) {
	fake.failedMutex.Lock()
	defer fake.failedMutex.Unlock()
	fake.FailedStub = nil
	fake.failedReturns = struct {
		result1 db.FailedContainer
		result2 error
	}{result1, result2}
}

func (fake *FakeCreatingContainer) FailedReturnsOnCall(i int, result1 db.FailedContainer, result2 error) {
	fake.failedMutex.Lock()
	defer fake.failedMutex.Unlock()
	fake.FailedStub = nil
	if fake.failedReturnsOnCall == nil {
		fake.failedReturnsOnCall = make(map[int]struct {
			result1 db.FailedContainer
			result2 error
		})
	}
	fake.failedReturnsOnCall[i] = struct {
		result1 db.FailedContainer
		result2 error
	}{result1, result2}
}

func (fake *FakeCreatingContainer) Handle() string {
	fake.handleMutex.Lock()
	ret, specificReturn := fake.handleReturnsOnCall[len(fake.handleArgsForCall)]
	fake.handleArgsForCall = append(fake.handleArgsForCall, struct {
	}{})
	stub := fake.HandleStub
	fakeReturns := fake.handleReturns
	fake.recordInvocation("Handle", []any{})
	fake.handleMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeCreatingContainer) HandleCallCount() int {
	fake.handleMutex.RLock()
	defer fake.handleMutex.RUnlock()
	return len(fake.handleArgsForCall)
}

func (fake *FakeCreatingContainer) HandleCalls(stub func() string) {
	fake.handleMutex.Lock()
	defer fake.handleMutex.Unlock()
	fake.HandleStub = stub
}

func (fake *FakeCreatingContainer) HandleReturns(result1 string) {
	fake.handleMutex.Lock()
	defer fake.handleMutex.Unlock()
	fake.HandleStub = nil
	fake.handleReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeCreatingContainer) HandleReturnsOnCall(i int, result1 string) {
	fake.handleMutex.Lock()
	defer fake.handleMutex.Unlock()
	fake.HandleStub = nil
	if fake.handleReturnsOnCall == nil {
		fake.handleReturnsOnCall = make(map[int]struct {
			result1 string
		})
	}
	fake.handleReturnsOnCall[i] = struct {
		result1 string
	}{result1}
}

func (fake *FakeCreatingContainer) ID() int {
	fake.iDMutex.Lock()
	ret, specificReturn := fake.iDReturnsOnCall[len(fake.iDArgsForCall)]
	fake.iDArgsForCall = append(fake.iDArgsForCall, struct {
	}{})
	stub := fake.IDStub
	fakeReturns := fake.iDReturns
	fake.recordInvocation("ID", []any{})
	fake.iDMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeCreatingContainer) IDCallCount() int {
	fake.iDMutex.RLock()
	defer fake.iDMutex.RUnlock()
	return len(fake.iDArgsForCall)
}

func (fake *FakeCreatingContainer) IDCalls(stub func() int) {
	fake.iDMutex.Lock()
	defer fake.iDMutex.Unlock()
	fake.IDStub = stub
}

func (fake *FakeCreatingContainer) IDReturns(result1 int) {
	fake.iDMutex.Lock()
	defer fake.iDMutex.Unlock()
	fake.IDStub = nil
	fake.iDReturns = struct {
		result1 int
	}{result1}
}

func (fake *FakeCreatingContainer) IDReturnsOnCall(i int, result1 int) {
	fake.iDMutex.Lock()
	defer fake.iDMutex.Unlock()
	fake.IDStub = nil
	if fake.iDReturnsOnCall == nil {
		fake.iDReturnsOnCall = make(map[int]struct {
			result1 int
		})
	}
	fake.iDReturnsOnCall[i] = struct {
		result1 int
	}{result1}
}

func (fake *FakeCreatingContainer) Metadata() db.ContainerMetadata {
	fake.metadataMutex.Lock()
	ret, specificReturn := fake.metadataReturnsOnCall[len(fake.metadataArgsForCall)]
	fake.metadataArgsForCall = append(fake.metadataArgsForCall, struct {
	}{})
	stub := fake.MetadataStub
	fakeReturns := fake.metadataReturns
	fake.recordInvocation("Metadata", []any{})
	fake.metadataMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeCreatingContainer) MetadataCallCount() int {
	fake.metadataMutex.RLock()
	defer fake.metadataMutex.RUnlock()
	return len(fake.metadataArgsForCall)
}

func (fake *FakeCreatingContainer) MetadataCalls(stub func() db.ContainerMetadata) {
	fake.metadataMutex.Lock()
	defer fake.metadataMutex.Unlock()
	fake.MetadataStub = stub
}

func (fake *FakeCreatingContainer) MetadataReturns(result1 db.ContainerMetadata) {
	fake.metadataMutex.Lock()
	defer fake.metadataMutex.Unlock()
	fake.MetadataStub = nil
	fake.metadataReturns = struct {
		result1 db.ContainerMetadata
	}{result1}
}

func (fake *FakeCreatingContainer) MetadataReturnsOnCall(i int, result1 db.ContainerMetadata) {
	fake.metadataMutex.Lock()
	defer fake.metadataMutex.Unlock()
	fake.MetadataStub = nil
	if fake.metadataReturnsOnCall == nil {
		fake.metadataReturnsOnCall = make(map[int]struct {
			result1 db.ContainerMetadata
		})
	}
	fake.metadataReturnsOnCall[i] = struct {
		result1 db.ContainerMetadata
	}{result1}
}

func (fake *FakeCreatingContainer) State() string {
	fake.stateMutex.Lock()
	ret, specificReturn := fake.stateReturnsOnCall[len(fake.stateArgsForCall)]
	fake.stateArgsForCall = append(fake.stateArgsForCall, struct {
	}{})
	stub := fake.StateStub
	fakeReturns := fake.stateReturns
	fake.recordInvocation("State", []any{})
	fake.stateMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeCreatingContainer) StateCallCount() int {
	fake.stateMutex.RLock()
	defer fake.stateMutex.RUnlock()
	return len(fake.stateArgsForCall)
}

func (fake *FakeCreatingContainer) StateCalls(stub func() string) {
	fake.stateMutex.Lock()
	defer fake.stateMutex.Unlock()
	fake.StateStub = stub
}

func (fake *FakeCreatingContainer) StateReturns(result1 string) {
	fake.stateMutex.Lock()
	defer fake.stateMutex.Unlock()
	fake.StateStub = nil
	fake.stateReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeCreatingContainer) StateReturnsOnCall(i int, result1 string) {
	fake.stateMutex.Lock()
	defer fake.stateMutex.Unlock()
	fake.StateStub = nil
	if fake.stateReturnsOnCall == nil {
		fake.stateReturnsOnCall = make(map[int]struct {
			result1 string
		})
	}
	fake.stateReturnsOnCall[i] = struct {
		result1 string
	}{result1}
}

func (fake *FakeCreatingContainer) WorkerName() string {
	fake.workerNameMutex.Lock()
	ret, specificReturn := fake.workerNameReturnsOnCall[len(fake.workerNameArgsForCall)]
	fake.workerNameArgsForCall = append(fake.workerNameArgsForCall, struct {
	}{})
	stub := fake.WorkerNameStub
	fakeReturns := fake.workerNameReturns
	fake.recordInvocation("WorkerName", []any{})
	fake.workerNameMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeCreatingContainer) WorkerNameCallCount() int {
	fake.workerNameMutex.RLock()
	defer fake.workerNameMutex.RUnlock()
	return len(fake.workerNameArgsForCall)
}

func (fake *FakeCreatingContainer) WorkerNameCalls(stub func() string) {
	fake.workerNameMutex.Lock()
	defer fake.workerNameMutex.Unlock()
	fake.WorkerNameStub = stub
}

func (fake *FakeCreatingContainer) WorkerNameReturns(result1 string) {
	fake.workerNameMutex.Lock()
	defer fake.workerNameMutex.Unlock()
	fake.WorkerNameStub = nil
	fake.workerNameReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeCreatingContainer) WorkerNameReturnsOnCall(i int, result1 string) {
	fake.workerNameMutex.Lock()
	defer fake.workerNameMutex.Unlock()
	fake.WorkerNameStub = nil
	if fake.workerNameReturnsOnCall == nil {
		fake.workerNameReturnsOnCall = make(map[int]struct {
			result1 string
		})
	}
	fake.workerNameReturnsOnCall[i] = struct {
		result1 string
	}{result1}
}

func (fake *FakeCreatingContainer) Invocations() map[string][][]any {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.createdMutex.RLock()
	defer fake.createdMutex.RUnlock()
	fake.failedMutex.RLock()
	defer fake.failedMutex.RUnlock()
	fake.handleMutex.RLock()
	defer fake.handleMutex.RUnlock()
	fake.iDMutex.RLock()
	defer fake.iDMutex.RUnlock()
	fake.metadataMutex.RLock()
	defer fake.metadataMutex.RUnlock()
	fake.stateMutex.RLock()
	defer fake.stateMutex.RUnlock()
	fake.workerNameMutex.RLock()
	defer fake.workerNameMutex.RUnlock()
	copiedInvocations := map[string][][]any{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeCreatingContainer) recordInvocation(key string, args []any) {
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

var _ db.CreatingContainer = new(FakeCreatingContainer)
