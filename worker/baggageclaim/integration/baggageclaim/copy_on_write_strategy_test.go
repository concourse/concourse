package integration_test

import (
	"github.com/concourse/concourse/worker/baggageclaim"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Copy On Write Strategy", func() {
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
		Describe("POST /volumes with strategy: cow", func() {
			It("creates a copy of the volume", func() {
				parentVolume, err := client.CreateVolume(ctx, "some-handle", baggageclaim.VolumeSpec{})
				Expect(err).NotTo(HaveOccurred())

				dataInParent := writeData(parentVolume.Path())
				Expect(dataExistsInVolume(dataInParent, parentVolume.Path())).To(BeTrue())

				childVolume, err := client.CreateVolume(ctx, "another-handle", baggageclaim.VolumeSpec{
					Strategy: baggageclaim.COWStrategy{
						Parent: parentVolume,
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(dataExistsInVolume(dataInParent, childVolume.Path())).To(BeTrue())

				newDataInParent := writeData(parentVolume.Path())
				Expect(dataExistsInVolume(newDataInParent, parentVolume.Path())).To(BeTrue())
				Expect(dataExistsInVolume(newDataInParent, childVolume.Path())).To(BeFalse())

				dataInChild := writeData(childVolume.Path())
				Expect(dataExistsInVolume(dataInChild, childVolume.Path())).To(BeTrue())
				Expect(dataExistsInVolume(dataInChild, parentVolume.Path())).To(BeFalse())
			})
		})
	})
})
