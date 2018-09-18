package helpers

import (
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/garden"
	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	groutes "code.cloudfoundry.org/garden/routes"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	bclient "github.com/concourse/baggageclaim/client"
	"github.com/concourse/go-concourse/concourse"
	"github.com/concourse/retryhttp"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/rata"
)

func WorkersAreLocal(logger lager.Logger, client concourse.Client) bool {
	workers, err := client.ListWorkers()
	Expect(err).NotTo(HaveOccurred())

	for _, w := range workers {
		if w.State != "running" || len(w.Tags) > 0 {
			continue
		}

		if strings.HasPrefix(w.GardenAddr, "127.") {
			return true
		}
	}

	return false
}

func WorkerWithResourceType(logger lager.Logger, client concourse.Client, resourceType string) (string, garden.Client, baggageclaim.Client) {
	gLog := logger.Session("garden-connection")

	workers, err := client.ListWorkers()
	Expect(err).NotTo(HaveOccurred())

	var rootfs string
	var gardenClient garden.Client
	var baggageclaimClient baggageclaim.Client

	for _, w := range workers {
		if w.State != "running" || len(w.Tags) > 0 {
			continue
		}

		rootfs = ""

		for _, r := range w.ResourceTypes {
			if r.Type == resourceType {
				rootfs = r.Image
			}
		}

		if rootfs == "" {
			continue
		}

		if strings.HasPrefix(w.GardenAddr, "127.") {
			ginkgo.Skip("worker is registered with local address; skipping")
		}

		backoff := retryhttp.NewExponentialBackOffFactory(5 * time.Minute)

		httpClient := &http.Client{
			Transport: &retryhttp.RetryRoundTripper{
				Logger:         logger.Session("retryable-http-client"),
				BackOffFactory: backoff,
				RoundTripper:   http.DefaultTransport,
				Retryer:        &retryhttp.DefaultRetryer{},
			},
		}

		hijackableClient := &retryhttp.RetryHijackableClient{
			Logger:           logger.Session("retry-hijackable-client"),
			BackOffFactory:   backoff,
			HijackableClient: retryhttp.DefaultHijackableClient,
			Retryer:          &retryhttp.DefaultRetryer{},
		}

		hijackStreamer := &WorkerHijackStreamer{
			HttpClient:       httpClient,
			HijackableClient: hijackableClient,
			Req:              rata.NewRequestGenerator("http://"+w.GardenAddr, groutes.Routes),
		}

		gardenClient = gclient.New(NewRetryableConnection(gconn.NewWithHijacker(hijackStreamer, gLog)))
		baggageclaimClient = bclient.New(w.BaggageclaimURL, http.DefaultTransport)

		break
	}

	if gardenClient == nil {
		ginkgo.Fail("must have at least one worker that supports " + resourceType + " resource type")
	}

	Eventually(gardenClient.Ping).Should(Succeed())

	return rootfs, gardenClient, baggageclaimClient
}
