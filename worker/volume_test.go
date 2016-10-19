package worker_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"
	bfakes "github.com/concourse/baggageclaim/baggageclaimfakes"
)

var _ = Describe("Volumes", func() {
	var (
		volumeFactory worker.VolumeFactory
		fakeVolume    *bfakes.FakeVolume
		fakeDB        *workerfakes.FakeVolumeFactoryDB
		logger        *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeVolume = new(bfakes.FakeVolume)
		fakeVolume.HandleReturns("some-handle")

		fakeDB = new(workerfakes.FakeVolumeFactoryDB)
		logger = lagertest.NewTestLogger("test")

		volumeFactory = worker.NewVolumeFactory(fakeDB)
	})

	Context("VolumeFactory", func() {
		Describe("BuildWithIndefiniteTTL", func() {
			Context("when the volume's TTL can be found", func() {
				BeforeEach(func() {
					fakeDB.GetVolumeTTLReturns(time.Minute, true, nil)
				})

				It("embeds the original volume in the wrapped volume", func() {
					vol, err := volumeFactory.BuildWithIndefiniteTTL(logger, fakeVolume)
					Expect(err).ToNot(HaveOccurred())
					Expect(vol.Handle()).To(Equal("some-handle"))
				})
			})
		})
	})
})
