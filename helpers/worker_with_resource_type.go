package helpers

import (
	"net/http"
	"strings"

	"code.cloudfoundry.org/garden"
	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	bclient "github.com/concourse/baggageclaim/client"
	"github.com/concourse/go-concourse/concourse"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

		gardenClient = gclient.New(gconn.NewWithLogger("tcp", w.GardenAddr, gLog))
		baggageclaimClient = bclient.New(w.BaggageclaimURL, http.DefaultTransport)

		break
	}

	if gardenClient == nil {
		ginkgo.Fail("must have at least one worker that supports " + resourceType + " resource type")
	}

	Eventually(gardenClient.Ping).Should(Succeed())

	return rootfs, gardenClient, baggageclaimClient
}
