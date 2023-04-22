package integration_test

import (
	"github.com/concourse/concourse/worker/baggageclaim"
	. "github.com/onsi/ginkgo/v2"
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
		emptyVolume, err := client.CreateVolume(ctx, "some-handle", baggageclaim.VolumeSpec{
			Properties: baggageclaim.VolumeProperties{
				"property-name": "property-value",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		err = emptyVolume.SetProperty(ctx, "another-property", "another-value")
		Expect(err).NotTo(HaveOccurred())

		someVolume, found, err := client.LookupVolume(ctx, emptyVolume.Handle())
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(someVolume.Properties(ctx)).To(Equal(baggageclaim.VolumeProperties{
			"property-name":    "property-value",
			"another-property": "another-value",
		}))

		err = someVolume.SetProperty(ctx, "another-property", "yet-another-value")
		Expect(err).NotTo(HaveOccurred())

		someVolume, found, err = client.LookupVolume(ctx, someVolume.Handle())
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		Expect(someVolume.Properties(ctx)).To(Equal(baggageclaim.VolumeProperties{
			"property-name":    "property-value",
			"another-property": "yet-another-value",
		}))

	})

	It("can find a volume by its properties", func() {
		_, err := client.CreateVolume(ctx, "some-handle-1", baggageclaim.VolumeSpec{})
		Expect(err).NotTo(HaveOccurred())

		emptyVolume, err := client.CreateVolume(ctx, "some-handle-2", baggageclaim.VolumeSpec{
			Properties: baggageclaim.VolumeProperties{
				"property-name": "property-value",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		err = emptyVolume.SetProperty(ctx, "another-property", "another-value")
		Expect(err).NotTo(HaveOccurred())

		foundVolumes, err := client.ListVolumes(ctx, baggageclaim.VolumeProperties{
			"another-property": "another-value",
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(foundVolumes).To(HaveLen(1))
		Expect(foundVolumes[0].Properties(ctx)).To(Equal(baggageclaim.VolumeProperties{
			"property-name":    "property-value",
			"another-property": "another-value",
		}))
	})

	It("returns ErrVolumeNotFound if the specified volume does not exist", func() {
		volume, err := client.CreateVolume(ctx, "some-handle", baggageclaim.VolumeSpec{})
		Expect(err).NotTo(HaveOccurred())

		Expect(volume.Destroy(ctx)).To(Succeed())

		err = volume.SetProperty(ctx, "some", "property")
		Expect(err).To(Equal(baggageclaim.ErrVolumeNotFound))
	})
})
