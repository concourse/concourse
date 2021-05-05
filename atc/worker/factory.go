package worker

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker/gardenruntime"
	"github.com/concourse/concourse/atc/worker/gclient"
	"github.com/concourse/concourse/atc/worker/transport"
	bclient "github.com/concourse/concourse/worker/baggageclaim/client"
	"github.com/concourse/retryhttp"
)

type Factory interface {
	NewWorker(lager.Logger, db.Worker) runtime.Worker
}

type DefaultFactory struct {
	DB DB

	GardenRequestTimeout              time.Duration
	BaggageclaimResponseHeaderTimeout time.Duration
	HTTPRetryTimeout                  time.Duration
	Compression                       compression.Compression
}

func (f DefaultFactory) NewWorker(logger lager.Logger, dbWorker db.Worker) runtime.Worker {
	return f.newGardenWorker(logger, dbWorker)
}

func (f DefaultFactory) newGardenWorker(logger lager.Logger, dbWorker db.Worker) *gardenruntime.Worker {
	gcf := gclient.NewGardenClientFactory(
		f.DB.WorkerFactory,
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
		f.DB.WorkerFactory,
		&http.Transport{
			DisableKeepAlives:     true,
			ResponseHeaderTimeout: f.BaggageclaimResponseHeaderTimeout,
		},
	))

	return gardenruntime.NewWorker(
		dbWorker,
		gClient,
		bcClient,
		f.DB.ToGardenRuntimeDB(),
		Streamer{Compression: f.Compression},
	)
}
