package fakechecker

import (
	"sync"

	"github.com/winston-ci/winston/config"
)

type FakeChecker struct {
	WhenCheckingResource func(config.Resource) []config.Resource
	checkedResources     []config.Resource

	sync.RWMutex
}

func New() *FakeChecker {
	return &FakeChecker{}
}

func (fake *FakeChecker) CheckResource(resource config.Resource) []config.Resource {
	if fake.WhenCheckingResource != nil {
		return fake.WhenCheckingResource(resource)
	}

	fake.Lock()
	fake.checkedResources = append(fake.checkedResources, resource)
	fake.Unlock()

	return nil
}

func (fake *FakeChecker) CheckedResources() []config.Resource {
	fake.RLock()
	defer fake.RUnlock()

	return fake.checkedResources
}
