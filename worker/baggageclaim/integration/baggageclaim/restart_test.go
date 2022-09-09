package integration_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/worker/baggageclaim"
)

var _ = Describe("Restarting", func() {
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

	It("can get volumes after the process restarts", func() {
		createdVolume, err := client.CreateVolume(ctx, "some-handle", baggageclaim.VolumeSpec{})
		Expect(err).NotTo(HaveOccurred())

		Expect(runner.CurrentHandles()).To(ConsistOf(createdVolume.Handle()))

		runner.Bounce()

		Expect(runner.CurrentHandles()).To(ConsistOf(createdVolume.Handle()))
	})
})
