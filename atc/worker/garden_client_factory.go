package worker

import (
	"net/http"

	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/garden/routes"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/v5/atc/worker/transport"
	"github.com/concourse/retryhttp"
	"github.com/tedsuo/rata"
)

type gardenClientFactory struct {
	db                  transport.TransportDB
	logger              lager.Logger
	workerName          string
	workerHost          *string
	retryBackOffFactory retryhttp.BackOffFactory
}

func NewGardenClientFactory(
	db transport.TransportDB,
	logger lager.Logger,
	workerName string,
	workerHost *string,
	retryBackOffFactory retryhttp.BackOffFactory,
) *gardenClientFactory {
	return &gardenClientFactory{
		db:                  db,
		logger:              logger,
		workerName:          workerName,
		workerHost:          workerHost,
		retryBackOffFactory: retryBackOffFactory,
	}
}

func (gcf *gardenClientFactory) NewClient() gclient.Client {
	retryer := &transport.UnreachableWorkerRetryer{
		DelegateRetryer: &retryhttp.DefaultRetryer{},
	}

	httpClient := &http.Client{
		Transport: &retryhttp.RetryRoundTripper{
			Logger:         gcf.logger.Session("retryable-http-client"),
			BackOffFactory: gcf.retryBackOffFactory,
			RoundTripper:   transport.NewGardenRoundTripper(gcf.workerName, gcf.workerHost, gcf.db, &http.Transport{DisableKeepAlives: true}),
			Retryer:        retryer,
		},
	}

	hijackableClient := &retryhttp.RetryHijackableClient{
		Logger:           gcf.logger.Session("retry-hijackable-client"),
		BackOffFactory:   gcf.retryBackOffFactory,
		HijackableClient: transport.NewHijackableClient(gcf.workerName, gcf.db, retryhttp.DefaultHijackableClient),
		Retryer:          retryer,
	}

	// the request generator's address doesn't matter because it's overwritten by the worker lookup clients
	hijackStreamer := &transport.WorkerHijackStreamer{
		HttpClient:       httpClient,
		HijackableClient: hijackableClient,
		Req:              rata.NewRequestGenerator("http://127.0.0.1:8080", routes.Routes),
	}

	return gclient.New(NewRetryableConnection(gconn.NewWithHijacker(hijackStreamer, gcf.logger)))
}
