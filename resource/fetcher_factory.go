package resource

import (
	"code.cloudfoundry.org/clock"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . FetcherFactory

type FetcherFactory interface {
	FetcherFor(workerClient worker.Client) Fetcher
}

func NewFetcherFactory(
	lockFactory lock.LockFactory,
	clock clock.Clock,
	dbResourceCacheFactory dbng.ResourceCacheFactory,
) FetcherFactory {
	return &fetcherFactory{
		lockFactory: lockFactory,
		clock:       clock,
		dbResourceCacheFactory: dbResourceCacheFactory,
	}
}

type fetcherFactory struct {
	lockFactory            lock.LockFactory
	clock                  clock.Clock
	dbResourceCacheFactory dbng.ResourceCacheFactory
}

func (f *fetcherFactory) FetcherFor(workerClient worker.Client) Fetcher {
	return NewFetcher(
		f.clock,
		f.lockFactory,
		NewFetchSourceProviderFactory(workerClient, f.dbResourceCacheFactory),
	)
}
