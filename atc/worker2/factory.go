package worker2

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	bclient "github.com/concourse/baggageclaim/client"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker/gclient"
	"github.com/concourse/concourse/atc/worker/transport"
	"github.com/concourse/concourse/atc/worker2/gardenruntime"
	"github.com/concourse/retryhttp"
)

type Factory interface {
	NewWorker(lager.Logger, Pool, db.Worker) runtime.Worker
}

type DefaultFactory struct {
	GardenRequestTimeout              time.Duration
	BaggageclaimResponseHeaderTimeout time.Duration
	HTTPRetryTimeout                  time.Duration
}

func (f DefaultFactory) NewWorker(logger lager.Logger, pool Pool, dbWorker db.Worker) runtime.Worker {
	return f.newGardenWorker(logger, pool, dbWorker)
}

func (f DefaultFactory) newGardenWorker(logger lager.Logger, pool Pool, dbWorker db.Worker) *gardenruntime.Worker {
	gcf := gclient.NewGardenClientFactory(
		pool.DB.WorkerFactory,
		logger.Session("garden-connection"),
		dbWorker.Name(),
		dbWorker.GardenAddr(),
		retryhttp.NewExponentialBackOffFactory(f.HTTPRetryTimeout),
		f.GardenRequestTimeout,
	)
	gClient := gcf.NewClient()
	bcClient := bclient.New("", transport.NewBaggageclaimRoundTripper(
		dbWorker.Name(),
		dbWorker.BaggageclaimURL(),
		pool.DB.WorkerFactory,
		&http.Transport{
			DisableKeepAlives:     true,
			ResponseHeaderTimeout: f.BaggageclaimResponseHeaderTimeout,
		},
	))

	return gardenruntime.NewWorker(
		dbWorker,
		gClient,
		bcClient,
		pool.DB.ToGardenRuntimeDB(),
		pool,
	)
}
