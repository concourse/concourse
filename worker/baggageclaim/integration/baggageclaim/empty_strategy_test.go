package integration_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/worker/baggageclaim"
)

var _ = Describe("Empty Strategy", func() {
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

	Describe("API", func() {
		properties := baggageclaim.VolumeProperties{
			"name": "value",
		}

		Describe("POST /volumes", func() {
			var (
				firstVolume baggageclaim.Volume
			)

			JustBeforeEach(func() {
				var err error
				firstVolume, err = client.CreateVolume(ctx, "some-handle", baggageclaim.VolumeSpec{})
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("created directory", func() {
				var (
					createdDir string
				)

				JustBeforeEach(func() {
					createdDir = firstVolume.Path()
				})

				It("is in the volume dir", func() {
					Expect(createdDir).To(HavePrefix(runner.VolumeDir()))
				})

				It("creates the directory", func() {
					Expect(createdDir).To(BeADirectory())
				})

				Context("on a second request", func() {
					var (
						secondVolume baggageclaim.Volume
					)

					JustBeforeEach(func() {
						var err error
						secondVolume, err = client.CreateVolume(ctx, "second-handle", baggageclaim.VolumeSpec{})
						Expect(err).NotTo(HaveOccurred())
					})

					It("creates a new directory", func() {
						Expect(createdDir).NotTo(Equal(secondVolume.Path()))
					})

					It("creates a new handle", func() {
						Expect(firstVolume.Handle).NotTo(Equal(secondVolume.Handle()))
					})
				})
			})
		})

		Describe("GET /volumes", func() {
			var (
				volumes baggageclaim.Volumes
			)

			JustBeforeEach(func() {
				var err error
				volumes, err = client.ListVolumes(ctx, baggageclaim.VolumeProperties{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an empty response", func() {
				Expect(volumes).To(BeEmpty())
			})

			Context("when a volume has been created", func() {
				var createdVolume baggageclaim.Volume

				BeforeEach(func() {
					var err error
					createdVolume, err = client.CreateVolume(ctx, "some-handle", baggageclaim.VolumeSpec{Properties: properties})
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns it", func() {
					Expect(runner.CurrentHandles()).To(ConsistOf(createdVolume.Handle()))
				})
			})
		})
	})
})
