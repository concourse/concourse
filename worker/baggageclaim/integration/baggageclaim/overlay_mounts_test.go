//go:build linux
// +build linux

package integration_test

import (
	"syscall"

	"github.com/concourse/concourse/worker/baggageclaim"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("baggageclaim restart", func() {

	var (
		runner *BaggageClaimRunner
		client baggageclaim.Client
	)

	BeforeEach(func() {
		runner = NewRunner(baggageClaimPath, "overlay")
		// Cannot use overlay driver if the overlays/volumes dir is fstype overlay.
		// This is because you can't nest overlay mounts ( a known limitation)
		// Mounting the TempDir as tmpfs lets us use the overlay driver for integration
		err := syscall.Mount("tmpfs", runner.volumeDir, "tmpfs", 0, "")
		Expect(err).NotTo(HaveOccurred())

		runner.Start()

		client = runner.Client()
	})

	AfterEach(func() {
		runner.Stop()

		err := syscall.Unmount(runner.volumeDir, 0)
		Expect(err).NotTo(HaveOccurred())

		runner.Cleanup()
	})

	Context("when overlay initialized volumes exist and the baggageclaim process restarts", func() {

		var (
			createdVolume       baggageclaim.Volume
			createdCOWVolume    baggageclaim.Volume
			createdCOWCOWVolume baggageclaim.Volume

			dataInParent string
			err          error
		)

		BeforeEach(func() {
			createdVolume, err = client.CreateVolume(ctx, "some-handle", baggageclaim.VolumeSpec{Strategy: baggageclaim.EmptyStrategy{}})
			Expect(err).NotTo(HaveOccurred())

			dataInParent = writeData(createdVolume.Path())
			Expect(dataExistsInVolume(dataInParent, createdVolume.Path())).To(BeTrue())

			createdCOWVolume, err = client.CreateVolume(
				ctx,
				"some-cow-handle",
				baggageclaim.VolumeSpec{
					Strategy:   baggageclaim.COWStrategy{Parent: createdVolume},
					Properties: map[string]string{},
					Privileged: false,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataExistsInVolume(dataInParent, createdCOWVolume.Path())).To(BeTrue())

			Expect(runner.CurrentHandles()).To(ConsistOf(
				createdVolume.Handle(),
				createdCOWVolume.Handle(),
			))

			createdCOWCOWVolume, err = client.CreateVolume(
				ctx,
				"some-cow-cow-handle",
				baggageclaim.VolumeSpec{
					Strategy:   baggageclaim.COWStrategy{Parent: createdCOWVolume},
					Properties: map[string]string{},
					Privileged: false,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataExistsInVolume(dataInParent, createdCOWCOWVolume.Path())).To(BeTrue())

			Expect(runner.CurrentHandles()).To(ConsistOf(
				createdVolume.Handle(),
				createdCOWVolume.Handle(),
				createdCOWCOWVolume.Handle(),
			))

			err = syscall.Unmount(createdVolume.Path(), 0)
			Expect(err).NotTo(HaveOccurred())
			err = syscall.Unmount(createdCOWVolume.Path(), 0)
			Expect(err).NotTo(HaveOccurred())
			err = syscall.Unmount(createdCOWCOWVolume.Path(), 0)
			Expect(err).NotTo(HaveOccurred())

			runner.Bounce()
		})

		AfterEach(func() {
			err = syscall.Unmount(createdVolume.Path(), 0)
			Expect(err).NotTo(HaveOccurred())
			err = syscall.Unmount(createdCOWVolume.Path(), 0)
			Expect(err).NotTo(HaveOccurred())
			err = syscall.Unmount(createdCOWCOWVolume.Path(), 0)
			Expect(err).NotTo(HaveOccurred())
		})

		It("the mounts between the overlays dir and the live volumes dir should be present", func() {
			Expect(runner.CurrentHandles()).To(ConsistOf(
				createdVolume.Handle(),
				createdCOWVolume.Handle(),
				createdCOWCOWVolume.Handle(),
			))

			Expect(dataExistsInVolume(dataInParent, createdVolume.Path())).To(BeTrue())
			Expect(dataExistsInVolume(dataInParent, createdCOWVolume.Path())).To(BeTrue())
			Expect(dataExistsInVolume(dataInParent, createdCOWCOWVolume.Path())).To(BeTrue())
		})
	})
})
