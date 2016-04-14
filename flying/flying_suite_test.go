package flying_test

import (
	"os"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	gclient "github.com/cloudfoundry-incubator/garden/client"
	gconn "github.com/cloudfoundry-incubator/garden/client/connection"

	"testing"
)

var (
	flyBin  string
	tmpHome string

	gardenClient garden.Client

	// needss git, curl
	gitServerRootfs string
)

var atcURL = helpers.AtcURL()
var targetedConcourse = "testflight"

var _ = SynchronizedBeforeSuite(func() []byte {
	Eventually(helpers.ErrorPolling(atcURL)).ShouldNot(HaveOccurred())

	data, err := helpers.FirstNodeFlySetup(atcURL, targetedConcourse)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	var err error
	flyBin, tmpHome, err = helpers.AllNodeFlySetup(data)
	Expect(err).NotTo(HaveOccurred())

	client, err := helpers.AllNodeClientSetup(data)
	Expect(err).NotTo(HaveOccurred())

	workers, err := client.ListWorkers()
	Expect(err).NotTo(HaveOccurred())

	logger := lagertest.NewTestLogger("testflight")

	gLog := logger.Session("garden-connection")

	for _, w := range workers {
		if len(w.Tags) > 0 {
			continue
		}

		gitServerRootfs = ""

		for _, r := range w.ResourceTypes {
			if r.Type == "git" {
				gitServerRootfs = r.Image
			}
		}

		if gitServerRootfs != "" {
			gardenClient = gclient.New(gconn.NewWithLogger("tcp", w.GardenAddr, gLog))
		}
	}

	if gitServerRootfs == "" {
		Fail("must have at least one worker that supports git and bosh-deployment resource types")
	}

})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	os.RemoveAll(tmpHome)
})

func TestFlying(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flying Suite")
}
