package volume_test

import (
	"github.com/concourse/concourse/worker/baggageclaim"
	"github.com/concourse/concourse/worker/baggageclaim/baggageclaimfakes"
	"github.com/concourse/concourse/worker/baggageclaim/volume"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Strategerizer", func() {
	var (
		strategerizer volume.Strategerizer
	)

	BeforeEach(func() {
		strategerizer = volume.NewStrategerizer()
	})

	Describe("StrategyFor", func() {
		var (
			request baggageclaim.VolumeRequest

			strategy       volume.Strategy
			strategyForErr error
		)

		BeforeEach(func() {
			request = baggageclaim.VolumeRequest{}
		})

		JustBeforeEach(func() {
			strategy, strategyForErr = strategerizer.StrategyFor(request)
		})

		Context("with an empty strategy", func() {
			BeforeEach(func() {
				request.Strategy = baggageclaim.EmptyStrategy{}.Encode()
			})

			It("succeeds", func() {
				Expect(strategyForErr).ToNot(HaveOccurred())
			})

			It("constructs an empty strategy", func() {
				Expect(strategy).To(Equal(volume.EmptyStrategy{}))
			})
		})

		Context("with an import strategy", func() {
			BeforeEach(func() {
				request.Strategy = baggageclaim.ImportStrategy{
					Path: "/some/host/path",
				}.Encode()
			})

			It("succeeds", func() {
				Expect(strategyForErr).ToNot(HaveOccurred())
			})

			It("constructs an import strategy", func() {
				Expect(strategy).To(Equal(volume.ImportStrategy{
					Path:           "/some/host/path",
					FollowSymlinks: false,
				}))
			})

			Context("when follow symlinks is set", func() {
				BeforeEach(func() {
					request.Strategy = baggageclaim.ImportStrategy{
						Path:           "/some/host/path",
						FollowSymlinks: true,
					}.Encode()
				})

				It("succeeds", func() {
					Expect(strategyForErr).ToNot(HaveOccurred())
				})

				It("constructs an import strategy", func() {
					Expect(strategy).To(Equal(volume.ImportStrategy{
						Path:           "/some/host/path",
						FollowSymlinks: true,
					}))
				})
			})
		})

		Context("with a COW strategy", func() {
			BeforeEach(func() {
				volume := new(baggageclaimfakes.FakeVolume)
				volume.HandleReturns("parent-handle")
				request.Strategy = baggageclaim.COWStrategy{Parent: volume}.Encode()
			})

			It("succeeds", func() {
				Expect(strategyForErr).ToNot(HaveOccurred())
			})

			It("constructs a COW strategy", func() {
				Expect(strategy).To(Equal(volume.COWStrategy{ParentHandle: "parent-handle"}))
			})
		})
	})
})
