package resource

import (
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . FetcherFactory

type FetcherFactory interface {
	FetcherFor(workerClient worker.Client) Fetcher
}

//go:generate counterfeiter . LockDB

type LockDB interface {
	GetTaskLock(logger lager.Logger, lockName string) (lock.Lock, bool, error)
}

func NewFetcherFactory(
	db LockDB,
	clock clock.Clock,
) FetcherFactory {
	return &fetcherFactory{
		db:    db,
		clock: clock,
	}
}

type fetcherFactory struct {
	db    LockDB
	clock clock.Clock
}

func (f *fetcherFactory) FetcherFor(workerClient worker.Client) Fetcher {
	return NewFetcher(
		f.clock,
		f.db,
		NewFetchContainerCreatorFactory(),
		NewFetchSourceProviderFactory(workerClient),
	)
}
