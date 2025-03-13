// Code generated by counterfeiter. DO NOT EDIT.
package dbfakes

import (
	"sync"
	"time"

	"github.com/concourse/concourse/atc/db"
)

type FakeUser struct {
	ConnectorStub        func() string
	connectorMutex       sync.RWMutex
	connectorArgsForCall []struct {
	}
	connectorReturns struct {
		result1 string
	}
	connectorReturnsOnCall map[int]struct {
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
	LastLoginStub        func() time.Time
	lastLoginMutex       sync.RWMutex
	lastLoginArgsForCall []struct {
	}
	lastLoginReturns struct {
		result1 time.Time
	}
	lastLoginReturnsOnCall map[int]struct {
		result1 time.Time
	}
	NameStub        func() string
	nameMutex       sync.RWMutex
	nameArgsForCall []struct {
	}
	nameReturns struct {
		result1 string
	}
	nameReturnsOnCall map[int]struct {
		result1 string
	}
	SubStub        func() string
	subMutex       sync.RWMutex
	subArgsForCall []struct {
	}
	subReturns struct {
		result1 string
	}
	subReturnsOnCall map[int]struct {
		result1 string
	}
	invocations      map[string][][]any
	invocationsMutex sync.RWMutex
}

func (fake *FakeUser) Connector() string {
	fake.connectorMutex.Lock()
	ret, specificReturn := fake.connectorReturnsOnCall[len(fake.connectorArgsForCall)]
	fake.connectorArgsForCall = append(fake.connectorArgsForCall, struct {
	}{})
	stub := fake.ConnectorStub
	fakeReturns := fake.connectorReturns
	fake.recordInvocation("Connector", []any{})
	fake.connectorMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeUser) ConnectorCallCount() int {
	fake.connectorMutex.RLock()
	defer fake.connectorMutex.RUnlock()
	return len(fake.connectorArgsForCall)
}

func (fake *FakeUser) ConnectorCalls(stub func() string) {
	fake.connectorMutex.Lock()
	defer fake.connectorMutex.Unlock()
	fake.ConnectorStub = stub
}

func (fake *FakeUser) ConnectorReturns(result1 string) {
	fake.connectorMutex.Lock()
	defer fake.connectorMutex.Unlock()
	fake.ConnectorStub = nil
	fake.connectorReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeUser) ConnectorReturnsOnCall(i int, result1 string) {
	fake.connectorMutex.Lock()
	defer fake.connectorMutex.Unlock()
	fake.ConnectorStub = nil
	if fake.connectorReturnsOnCall == nil {
		fake.connectorReturnsOnCall = make(map[int]struct {
			result1 string
		})
	}
	fake.connectorReturnsOnCall[i] = struct {
		result1 string
	}{result1}
}

func (fake *FakeUser) ID() int {
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

func (fake *FakeUser) IDCallCount() int {
	fake.iDMutex.RLock()
	defer fake.iDMutex.RUnlock()
	return len(fake.iDArgsForCall)
}

func (fake *FakeUser) IDCalls(stub func() int) {
	fake.iDMutex.Lock()
	defer fake.iDMutex.Unlock()
	fake.IDStub = stub
}

func (fake *FakeUser) IDReturns(result1 int) {
	fake.iDMutex.Lock()
	defer fake.iDMutex.Unlock()
	fake.IDStub = nil
	fake.iDReturns = struct {
		result1 int
	}{result1}
}

func (fake *FakeUser) IDReturnsOnCall(i int, result1 int) {
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

func (fake *FakeUser) LastLogin() time.Time {
	fake.lastLoginMutex.Lock()
	ret, specificReturn := fake.lastLoginReturnsOnCall[len(fake.lastLoginArgsForCall)]
	fake.lastLoginArgsForCall = append(fake.lastLoginArgsForCall, struct {
	}{})
	stub := fake.LastLoginStub
	fakeReturns := fake.lastLoginReturns
	fake.recordInvocation("LastLogin", []any{})
	fake.lastLoginMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeUser) LastLoginCallCount() int {
	fake.lastLoginMutex.RLock()
	defer fake.lastLoginMutex.RUnlock()
	return len(fake.lastLoginArgsForCall)
}

func (fake *FakeUser) LastLoginCalls(stub func() time.Time) {
	fake.lastLoginMutex.Lock()
	defer fake.lastLoginMutex.Unlock()
	fake.LastLoginStub = stub
}

func (fake *FakeUser) LastLoginReturns(result1 time.Time) {
	fake.lastLoginMutex.Lock()
	defer fake.lastLoginMutex.Unlock()
	fake.LastLoginStub = nil
	fake.lastLoginReturns = struct {
		result1 time.Time
	}{result1}
}

func (fake *FakeUser) LastLoginReturnsOnCall(i int, result1 time.Time) {
	fake.lastLoginMutex.Lock()
	defer fake.lastLoginMutex.Unlock()
	fake.LastLoginStub = nil
	if fake.lastLoginReturnsOnCall == nil {
		fake.lastLoginReturnsOnCall = make(map[int]struct {
			result1 time.Time
		})
	}
	fake.lastLoginReturnsOnCall[i] = struct {
		result1 time.Time
	}{result1}
}

func (fake *FakeUser) Name() string {
	fake.nameMutex.Lock()
	ret, specificReturn := fake.nameReturnsOnCall[len(fake.nameArgsForCall)]
	fake.nameArgsForCall = append(fake.nameArgsForCall, struct {
	}{})
	stub := fake.NameStub
	fakeReturns := fake.nameReturns
	fake.recordInvocation("Name", []any{})
	fake.nameMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeUser) NameCallCount() int {
	fake.nameMutex.RLock()
	defer fake.nameMutex.RUnlock()
	return len(fake.nameArgsForCall)
}

func (fake *FakeUser) NameCalls(stub func() string) {
	fake.nameMutex.Lock()
	defer fake.nameMutex.Unlock()
	fake.NameStub = stub
}

func (fake *FakeUser) NameReturns(result1 string) {
	fake.nameMutex.Lock()
	defer fake.nameMutex.Unlock()
	fake.NameStub = nil
	fake.nameReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeUser) NameReturnsOnCall(i int, result1 string) {
	fake.nameMutex.Lock()
	defer fake.nameMutex.Unlock()
	fake.NameStub = nil
	if fake.nameReturnsOnCall == nil {
		fake.nameReturnsOnCall = make(map[int]struct {
			result1 string
		})
	}
	fake.nameReturnsOnCall[i] = struct {
		result1 string
	}{result1}
}

func (fake *FakeUser) Sub() string {
	fake.subMutex.Lock()
	ret, specificReturn := fake.subReturnsOnCall[len(fake.subArgsForCall)]
	fake.subArgsForCall = append(fake.subArgsForCall, struct {
	}{})
	stub := fake.SubStub
	fakeReturns := fake.subReturns
	fake.recordInvocation("Sub", []any{})
	fake.subMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeUser) SubCallCount() int {
	fake.subMutex.RLock()
	defer fake.subMutex.RUnlock()
	return len(fake.subArgsForCall)
}

func (fake *FakeUser) SubCalls(stub func() string) {
	fake.subMutex.Lock()
	defer fake.subMutex.Unlock()
	fake.SubStub = stub
}

func (fake *FakeUser) SubReturns(result1 string) {
	fake.subMutex.Lock()
	defer fake.subMutex.Unlock()
	fake.SubStub = nil
	fake.subReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeUser) SubReturnsOnCall(i int, result1 string) {
	fake.subMutex.Lock()
	defer fake.subMutex.Unlock()
	fake.SubStub = nil
	if fake.subReturnsOnCall == nil {
		fake.subReturnsOnCall = make(map[int]struct {
			result1 string
		})
	}
	fake.subReturnsOnCall[i] = struct {
		result1 string
	}{result1}
}

func (fake *FakeUser) Invocations() map[string][][]any {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.connectorMutex.RLock()
	defer fake.connectorMutex.RUnlock()
	fake.iDMutex.RLock()
	defer fake.iDMutex.RUnlock()
	fake.lastLoginMutex.RLock()
	defer fake.lastLoginMutex.RUnlock()
	fake.nameMutex.RLock()
	defer fake.nameMutex.RUnlock()
	fake.subMutex.RLock()
	defer fake.subMutex.RUnlock()
	copiedInvocations := map[string][][]any{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeUser) recordInvocation(key string, args []any) {
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

var _ db.User = new(FakeUser)
