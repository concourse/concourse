package gclient

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/garden/routes"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/worker/gclient/connection"
	"github.com/concourse/concourse/atc/worker/transport"
	"github.com/concourse/retryhttp"
	"github.com/tedsuo/rata"
)

type gardenClientFactory struct {
	db                         transport.TransportDB
	logger                     lager.Logger
	workerName                 string
	workerHost                 *string
	retryBackOffFactory        retryhttp.BackOffFactory
	streamClientRequestTimeout time.Duration
}

func NewGardenClientFactory(
	db transport.TransportDB,
	logger lager.Logger,
	workerName string,
	workerHost *string,
	retryBackOffFactory retryhttp.BackOffFactory,
	streamClientRequestTimeout time.Duration,
) *gardenClientFactory {
	return &gardenClientFactory{
		db:                         db,
		logger:                     logger,
		workerName:                 workerName,
		workerHost:                 workerHost,
		retryBackOffFactory:        retryBackOffFactory,
		streamClientRequestTimeout: streamClientRequestTimeout,
	}
}

func (gcf *gardenClientFactory) NewClient() Client {
	retryer := &transport.UnreachableWorkerRetryer{
		DelegateRetryer: &retryhttp.DefaultRetryer{},
	}

	streamClient := &http.Client{
		Transport: &retryhttp.RetryRoundTripper{
			Logger:         gcf.logger.Session("retryable-http-client"),
			BackOffFactory: gcf.retryBackOffFactory,
			RoundTripper:   transport.NewGardenRoundTripper(gcf.workerName, gcf.workerHost, gcf.db, &http.Transport{DisableKeepAlives: true}),
			Retryer:        retryer,
		},
		Timeout: gcf.streamClientRequestTimeout,
	}

	hijackableClient := &retryhttp.RetryHijackableClient{
		Logger:           gcf.logger.Session("retry-hijackable-client"),
		BackOffFactory:   gcf.retryBackOffFactory,
		HijackableClient: transport.NewHijackableClient(gcf.workerName, gcf.db, retryhttp.DefaultHijackableClient),
		Retryer:          retryer,
	}

	// the request generator's address doesn't matter because it's overwritten by the worker lookup clients
	hijackStreamer := &transport.WorkerHijackStreamer{
		HttpClient:       streamClient,
		HijackableClient: hijackableClient,
		Req:              rata.NewRequestGenerator("http://127.0.0.1:8080", routes.Routes),
	}

	return NewClient(NewRetryableConnection(connection.NewWithHijacker(hijackStreamer, gcf.logger)))
}

// Do not try any client method that requires hijack functionality (streaming logs)!
func BasicGardenClientWithRequestTimeout(logger lager.Logger, requestTimeout time.Duration, address string) Client {
	streamClient := &http.Client{
		Timeout: requestTimeout,
	}

	streamer := &transport.WorkerHijackStreamer{
		HttpClient:       streamClient,
		HijackableClient: nil,
		Req:              rata.NewRequestGenerator(address, routes.Routes),
	}

	return NewClient(connection.NewWithHijacker(streamer, logger))
}
