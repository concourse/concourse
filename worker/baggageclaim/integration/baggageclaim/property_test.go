package integration_test

import (
	"github.com/concourse/concourse/worker/baggageclaim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Properties", func() {
	var (
		runner *BaggageClaimRunner
		client baggageclaim.Client
	)

	BeforeEach(func() {
		runner = NewRunner(baggageClaimPath, "naive")
		runner.Start()

		client = runner.Client()
	})

	AfterEach(func() {
		runner.Stop()
		runner.Cleanup()
	})

	It("can manage properties", func() {
		emptyVolume, err := client.CreateVolume(logger, "some-handle", baggageclaim.VolumeSpec{
			Properties: baggageclaim.VolumeProperties{
				"property-name": "property-value",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		err = emptyVolume.SetProperty("another-property", "another-value")
		Expect(err).NotTo(HaveOccurred())

		someVolume, found, err := client.LookupVolume(logger, emptyVolume.Handle())
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(someVolume.Properties()).To(Equal(baggageclaim.VolumeProperties{
			"property-name":    "property-value",
			"another-property": "another-value",
		}))

		err = someVolume.SetProperty("another-property", "yet-another-value")
		Expect(err).NotTo(HaveOccurred())

		someVolume, found, err = client.LookupVolume(logger, someVolume.Handle())
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(someVolume.Properties()).To(Equal(baggageclaim.VolumeProperties{
			"property-name":    "property-value",
			"another-property": "yet-another-value",
		}))

	})

	It("can find a volume by its properties", func() {
		_, err := client.CreateVolume(logger, "some-handle-1", baggageclaim.VolumeSpec{})
		Expect(err).NotTo(HaveOccurred())

		emptyVolume, err := client.CreateVolume(logger, "some-handle-2", baggageclaim.VolumeSpec{
			Properties: baggageclaim.VolumeProperties{
				"property-name": "property-value",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		err = emptyVolume.SetProperty("another-property", "another-value")
		Expect(err).NotTo(HaveOccurred())

		foundVolumes, err := client.ListVolumes(logger, baggageclaim.VolumeProperties{
			"another-property": "another-value",
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(foundVolumes).To(HaveLen(1))
		Expect(foundVolumes[0].Properties()).To(Equal(baggageclaim.VolumeProperties{
			"property-name":    "property-value",
			"another-property": "another-value",
		}))
	})

	It("returns ErrVolumeNotFound if the specified volume does not exist", func() {
		volume, err := client.CreateVolume(logger, "some-handle", baggageclaim.VolumeSpec{})
		Expect(err).NotTo(HaveOccurred())

		Expect(volume.Destroy()).To(Succeed())

		err = volume.SetProperty("some", "property")
		Expect(err).To(Equal(baggageclaim.ErrVolumeNotFound))
	})
})
