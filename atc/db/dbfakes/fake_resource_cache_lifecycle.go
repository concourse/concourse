// Code generated by counterfeiter. DO NOT EDIT.
package dbfakes

import (
	"sync"

	lager "code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/db"
)

type FakeResourceCacheLifecycle struct {
	CleanBuildImageResourceCachesStub        func(lager.Logger) error
	cleanBuildImageResourceCachesMutex       sync.RWMutex
	cleanBuildImageResourceCachesArgsForCall []struct {
		arg1 lager.Logger
	}
	cleanBuildImageResourceCachesReturns struct {
		result1 error
	}
	cleanBuildImageResourceCachesReturnsOnCall map[int]struct {
		result1 error
	}
	CleanDirtyInMemoryBuildUsesStub        func(lager.Logger) error
	cleanDirtyInMemoryBuildUsesMutex       sync.RWMutex
	cleanDirtyInMemoryBuildUsesArgsForCall []struct {
		arg1 lager.Logger
	}
	cleanDirtyInMemoryBuildUsesReturns struct {
		result1 error
	}
	cleanDirtyInMemoryBuildUsesReturnsOnCall map[int]struct {
		result1 error
	}
	CleanInvalidWorkerResourceCachesStub        func(lager.Logger, int) error
	cleanInvalidWorkerResourceCachesMutex       sync.RWMutex
	cleanInvalidWorkerResourceCachesArgsForCall []struct {
		arg1 lager.Logger
		arg2 int
	}
	cleanInvalidWorkerResourceCachesReturns struct {
		result1 error
	}
	cleanInvalidWorkerResourceCachesReturnsOnCall map[int]struct {
		result1 error
	}
	CleanUpInvalidCachesStub        func(lager.Logger) error
	cleanUpInvalidCachesMutex       sync.RWMutex
	cleanUpInvalidCachesArgsForCall []struct {
		arg1 lager.Logger
	}
	cleanUpInvalidCachesReturns struct {
		result1 error
	}
	cleanUpInvalidCachesReturnsOnCall map[int]struct {
		result1 error
	}
	CleanUsesForFinishedBuildsStub        func(lager.Logger) error
	cleanUsesForFinishedBuildsMutex       sync.RWMutex
	cleanUsesForFinishedBuildsArgsForCall []struct {
		arg1 lager.Logger
	}
	cleanUsesForFinishedBuildsReturns struct {
		result1 error
	}
	cleanUsesForFinishedBuildsReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]any
	invocationsMutex sync.RWMutex
}

func (fake *FakeResourceCacheLifecycle) CleanBuildImageResourceCaches(arg1 lager.Logger) error {
	fake.cleanBuildImageResourceCachesMutex.Lock()
	ret, specificReturn := fake.cleanBuildImageResourceCachesReturnsOnCall[len(fake.cleanBuildImageResourceCachesArgsForCall)]
	fake.cleanBuildImageResourceCachesArgsForCall = append(fake.cleanBuildImageResourceCachesArgsForCall, struct {
		arg1 lager.Logger
	}{arg1})
	stub := fake.CleanBuildImageResourceCachesStub
	fakeReturns := fake.cleanBuildImageResourceCachesReturns
	fake.recordInvocation("CleanBuildImageResourceCaches", []any{arg1})
	fake.cleanBuildImageResourceCachesMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeResourceCacheLifecycle) CleanBuildImageResourceCachesCallCount() int {
	fake.cleanBuildImageResourceCachesMutex.RLock()
	defer fake.cleanBuildImageResourceCachesMutex.RUnlock()
	return len(fake.cleanBuildImageResourceCachesArgsForCall)
}

func (fake *FakeResourceCacheLifecycle) CleanBuildImageResourceCachesCalls(stub func(lager.Logger) error) {
	fake.cleanBuildImageResourceCachesMutex.Lock()
	defer fake.cleanBuildImageResourceCachesMutex.Unlock()
	fake.CleanBuildImageResourceCachesStub = stub
}

func (fake *FakeResourceCacheLifecycle) CleanBuildImageResourceCachesArgsForCall(i int) lager.Logger {
	fake.cleanBuildImageResourceCachesMutex.RLock()
	defer fake.cleanBuildImageResourceCachesMutex.RUnlock()
	argsForCall := fake.cleanBuildImageResourceCachesArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeResourceCacheLifecycle) CleanBuildImageResourceCachesReturns(result1 error) {
	fake.cleanBuildImageResourceCachesMutex.Lock()
	defer fake.cleanBuildImageResourceCachesMutex.Unlock()
	fake.CleanBuildImageResourceCachesStub = nil
	fake.cleanBuildImageResourceCachesReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeResourceCacheLifecycle) CleanBuildImageResourceCachesReturnsOnCall(i int, result1 error) {
	fake.cleanBuildImageResourceCachesMutex.Lock()
	defer fake.cleanBuildImageResourceCachesMutex.Unlock()
	fake.CleanBuildImageResourceCachesStub = nil
	if fake.cleanBuildImageResourceCachesReturnsOnCall == nil {
		fake.cleanBuildImageResourceCachesReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.cleanBuildImageResourceCachesReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeResourceCacheLifecycle) CleanDirtyInMemoryBuildUses(arg1 lager.Logger) error {
	fake.cleanDirtyInMemoryBuildUsesMutex.Lock()
	ret, specificReturn := fake.cleanDirtyInMemoryBuildUsesReturnsOnCall[len(fake.cleanDirtyInMemoryBuildUsesArgsForCall)]
	fake.cleanDirtyInMemoryBuildUsesArgsForCall = append(fake.cleanDirtyInMemoryBuildUsesArgsForCall, struct {
		arg1 lager.Logger
	}{arg1})
	stub := fake.CleanDirtyInMemoryBuildUsesStub
	fakeReturns := fake.cleanDirtyInMemoryBuildUsesReturns
	fake.recordInvocation("CleanDirtyInMemoryBuildUses", []any{arg1})
	fake.cleanDirtyInMemoryBuildUsesMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeResourceCacheLifecycle) CleanDirtyInMemoryBuildUsesCallCount() int {
	fake.cleanDirtyInMemoryBuildUsesMutex.RLock()
	defer fake.cleanDirtyInMemoryBuildUsesMutex.RUnlock()
	return len(fake.cleanDirtyInMemoryBuildUsesArgsForCall)
}

func (fake *FakeResourceCacheLifecycle) CleanDirtyInMemoryBuildUsesCalls(stub func(lager.Logger) error) {
	fake.cleanDirtyInMemoryBuildUsesMutex.Lock()
	defer fake.cleanDirtyInMemoryBuildUsesMutex.Unlock()
	fake.CleanDirtyInMemoryBuildUsesStub = stub
}

func (fake *FakeResourceCacheLifecycle) CleanDirtyInMemoryBuildUsesArgsForCall(i int) lager.Logger {
	fake.cleanDirtyInMemoryBuildUsesMutex.RLock()
	defer fake.cleanDirtyInMemoryBuildUsesMutex.RUnlock()
	argsForCall := fake.cleanDirtyInMemoryBuildUsesArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeResourceCacheLifecycle) CleanDirtyInMemoryBuildUsesReturns(result1 error) {
	fake.cleanDirtyInMemoryBuildUsesMutex.Lock()
	defer fake.cleanDirtyInMemoryBuildUsesMutex.Unlock()
	fake.CleanDirtyInMemoryBuildUsesStub = nil
	fake.cleanDirtyInMemoryBuildUsesReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeResourceCacheLifecycle) CleanDirtyInMemoryBuildUsesReturnsOnCall(i int, result1 error) {
	fake.cleanDirtyInMemoryBuildUsesMutex.Lock()
	defer fake.cleanDirtyInMemoryBuildUsesMutex.Unlock()
	fake.CleanDirtyInMemoryBuildUsesStub = nil
	if fake.cleanDirtyInMemoryBuildUsesReturnsOnCall == nil {
		fake.cleanDirtyInMemoryBuildUsesReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.cleanDirtyInMemoryBuildUsesReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeResourceCacheLifecycle) CleanInvalidWorkerResourceCaches(arg1 lager.Logger, arg2 int) error {
	fake.cleanInvalidWorkerResourceCachesMutex.Lock()
	ret, specificReturn := fake.cleanInvalidWorkerResourceCachesReturnsOnCall[len(fake.cleanInvalidWorkerResourceCachesArgsForCall)]
	fake.cleanInvalidWorkerResourceCachesArgsForCall = append(fake.cleanInvalidWorkerResourceCachesArgsForCall, struct {
		arg1 lager.Logger
		arg2 int
	}{arg1, arg2})
	stub := fake.CleanInvalidWorkerResourceCachesStub
	fakeReturns := fake.cleanInvalidWorkerResourceCachesReturns
	fake.recordInvocation("CleanInvalidWorkerResourceCaches", []any{arg1, arg2})
	fake.cleanInvalidWorkerResourceCachesMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeResourceCacheLifecycle) CleanInvalidWorkerResourceCachesCallCount() int {
	fake.cleanInvalidWorkerResourceCachesMutex.RLock()
	defer fake.cleanInvalidWorkerResourceCachesMutex.RUnlock()
	return len(fake.cleanInvalidWorkerResourceCachesArgsForCall)
}

func (fake *FakeResourceCacheLifecycle) CleanInvalidWorkerResourceCachesCalls(stub func(lager.Logger, int) error) {
	fake.cleanInvalidWorkerResourceCachesMutex.Lock()
	defer fake.cleanInvalidWorkerResourceCachesMutex.Unlock()
	fake.CleanInvalidWorkerResourceCachesStub = stub
}

func (fake *FakeResourceCacheLifecycle) CleanInvalidWorkerResourceCachesArgsForCall(i int) (lager.Logger, int) {
	fake.cleanInvalidWorkerResourceCachesMutex.RLock()
	defer fake.cleanInvalidWorkerResourceCachesMutex.RUnlock()
	argsForCall := fake.cleanInvalidWorkerResourceCachesArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeResourceCacheLifecycle) CleanInvalidWorkerResourceCachesReturns(result1 error) {
	fake.cleanInvalidWorkerResourceCachesMutex.Lock()
	defer fake.cleanInvalidWorkerResourceCachesMutex.Unlock()
	fake.CleanInvalidWorkerResourceCachesStub = nil
	fake.cleanInvalidWorkerResourceCachesReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeResourceCacheLifecycle) CleanInvalidWorkerResourceCachesReturnsOnCall(i int, result1 error) {
	fake.cleanInvalidWorkerResourceCachesMutex.Lock()
	defer fake.cleanInvalidWorkerResourceCachesMutex.Unlock()
	fake.CleanInvalidWorkerResourceCachesStub = nil
	if fake.cleanInvalidWorkerResourceCachesReturnsOnCall == nil {
		fake.cleanInvalidWorkerResourceCachesReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.cleanInvalidWorkerResourceCachesReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeResourceCacheLifecycle) CleanUpInvalidCaches(arg1 lager.Logger) error {
	fake.cleanUpInvalidCachesMutex.Lock()
	ret, specificReturn := fake.cleanUpInvalidCachesReturnsOnCall[len(fake.cleanUpInvalidCachesArgsForCall)]
	fake.cleanUpInvalidCachesArgsForCall = append(fake.cleanUpInvalidCachesArgsForCall, struct {
		arg1 lager.Logger
	}{arg1})
	stub := fake.CleanUpInvalidCachesStub
	fakeReturns := fake.cleanUpInvalidCachesReturns
	fake.recordInvocation("CleanUpInvalidCaches", []any{arg1})
	fake.cleanUpInvalidCachesMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeResourceCacheLifecycle) CleanUpInvalidCachesCallCount() int {
	fake.cleanUpInvalidCachesMutex.RLock()
	defer fake.cleanUpInvalidCachesMutex.RUnlock()
	return len(fake.cleanUpInvalidCachesArgsForCall)
}

func (fake *FakeResourceCacheLifecycle) CleanUpInvalidCachesCalls(stub func(lager.Logger) error) {
	fake.cleanUpInvalidCachesMutex.Lock()
	defer fake.cleanUpInvalidCachesMutex.Unlock()
	fake.CleanUpInvalidCachesStub = stub
}

func (fake *FakeResourceCacheLifecycle) CleanUpInvalidCachesArgsForCall(i int) lager.Logger {
	fake.cleanUpInvalidCachesMutex.RLock()
	defer fake.cleanUpInvalidCachesMutex.RUnlock()
	argsForCall := fake.cleanUpInvalidCachesArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeResourceCacheLifecycle) CleanUpInvalidCachesReturns(result1 error) {
	fake.cleanUpInvalidCachesMutex.Lock()
	defer fake.cleanUpInvalidCachesMutex.Unlock()
	fake.CleanUpInvalidCachesStub = nil
	fake.cleanUpInvalidCachesReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeResourceCacheLifecycle) CleanUpInvalidCachesReturnsOnCall(i int, result1 error) {
	fake.cleanUpInvalidCachesMutex.Lock()
	defer fake.cleanUpInvalidCachesMutex.Unlock()
	fake.CleanUpInvalidCachesStub = nil
	if fake.cleanUpInvalidCachesReturnsOnCall == nil {
		fake.cleanUpInvalidCachesReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.cleanUpInvalidCachesReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeResourceCacheLifecycle) CleanUsesForFinishedBuilds(arg1 lager.Logger) error {
	fake.cleanUsesForFinishedBuildsMutex.Lock()
	ret, specificReturn := fake.cleanUsesForFinishedBuildsReturnsOnCall[len(fake.cleanUsesForFinishedBuildsArgsForCall)]
	fake.cleanUsesForFinishedBuildsArgsForCall = append(fake.cleanUsesForFinishedBuildsArgsForCall, struct {
		arg1 lager.Logger
	}{arg1})
	stub := fake.CleanUsesForFinishedBuildsStub
	fakeReturns := fake.cleanUsesForFinishedBuildsReturns
	fake.recordInvocation("CleanUsesForFinishedBuilds", []any{arg1})
	fake.cleanUsesForFinishedBuildsMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeResourceCacheLifecycle) CleanUsesForFinishedBuildsCallCount() int {
	fake.cleanUsesForFinishedBuildsMutex.RLock()
	defer fake.cleanUsesForFinishedBuildsMutex.RUnlock()
	return len(fake.cleanUsesForFinishedBuildsArgsForCall)
}

func (fake *FakeResourceCacheLifecycle) CleanUsesForFinishedBuildsCalls(stub func(lager.Logger) error) {
	fake.cleanUsesForFinishedBuildsMutex.Lock()
	defer fake.cleanUsesForFinishedBuildsMutex.Unlock()
	fake.CleanUsesForFinishedBuildsStub = stub
}

func (fake *FakeResourceCacheLifecycle) CleanUsesForFinishedBuildsArgsForCall(i int) lager.Logger {
	fake.cleanUsesForFinishedBuildsMutex.RLock()
	defer fake.cleanUsesForFinishedBuildsMutex.RUnlock()
	argsForCall := fake.cleanUsesForFinishedBuildsArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeResourceCacheLifecycle) CleanUsesForFinishedBuildsReturns(result1 error) {
	fake.cleanUsesForFinishedBuildsMutex.Lock()
	defer fake.cleanUsesForFinishedBuildsMutex.Unlock()
	fake.CleanUsesForFinishedBuildsStub = nil
	fake.cleanUsesForFinishedBuildsReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeResourceCacheLifecycle) CleanUsesForFinishedBuildsReturnsOnCall(i int, result1 error) {
	fake.cleanUsesForFinishedBuildsMutex.Lock()
	defer fake.cleanUsesForFinishedBuildsMutex.Unlock()
	fake.CleanUsesForFinishedBuildsStub = nil
	if fake.cleanUsesForFinishedBuildsReturnsOnCall == nil {
		fake.cleanUsesForFinishedBuildsReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.cleanUsesForFinishedBuildsReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeResourceCacheLifecycle) Invocations() map[string][][]any {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.cleanBuildImageResourceCachesMutex.RLock()
	defer fake.cleanBuildImageResourceCachesMutex.RUnlock()
	fake.cleanDirtyInMemoryBuildUsesMutex.RLock()
	defer fake.cleanDirtyInMemoryBuildUsesMutex.RUnlock()
	fake.cleanInvalidWorkerResourceCachesMutex.RLock()
	defer fake.cleanInvalidWorkerResourceCachesMutex.RUnlock()
	fake.cleanUpInvalidCachesMutex.RLock()
	defer fake.cleanUpInvalidCachesMutex.RUnlock()
	fake.cleanUsesForFinishedBuildsMutex.RLock()
	defer fake.cleanUsesForFinishedBuildsMutex.RUnlock()
	copiedInvocations := map[string][][]any{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeResourceCacheLifecycle) recordInvocation(key string, args []any) {
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

var _ db.ResourceCacheLifecycle = new(FakeResourceCacheLifecycle)
