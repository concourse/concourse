package fakechecker

import (
	"sync"

	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
)

type FakeChecker struct {
	WhenCheckingResource func(config.Resource, builds.Version) []builds.Version
	checked              []CheckedSpec

	sync.RWMutex
}

type CheckedSpec struct {
	Resource config.Resource
	Version  builds.Version
}

func New() *FakeChecker {
	return &FakeChecker{}
}

func (fake *FakeChecker) CheckResource(resource config.Resource, version builds.Version) []builds.Version {
	if fake.WhenCheckingResource != nil {
		return fake.WhenCheckingResource(resource, version)
	}

	fake.Lock()
	fake.checked = append(fake.checked, CheckedSpec{resource, version})
	fake.Unlock()

	return nil
}

func (fake *FakeChecker) Checked() []CheckedSpec {
	fake.RLock()
	defer fake.RUnlock()

	return fake.checked
}
