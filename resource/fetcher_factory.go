package resource

import (
	"time"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . FetcherFactory

type FetcherFactory interface {
	FetcherFor(workerClient worker.Client) Fetcher
}

//go:generate counterfeiter . LeaseDB

type LeaseDB interface {
	GetLease(logger lager.Logger, leaseName string, interval time.Duration) (db.Lease, bool, error)
}

func NewFetcherFactory(
	db LeaseDB,
	clock clock.Clock,
) FetcherFactory {
	return &fetcherFactory{
		db:    db,
		clock: clock,
	}
}

type fetcherFactory struct {
	db    LeaseDB
	clock clock.Clock
}

func (f *fetcherFactory) FetcherFor(workerClient worker.Client) Fetcher {
	return NewFetcher(f.clock, f.db, NewFetchContainerCreatorFactory(), NewFetchSourceProviderFactory(workerClient))
}
