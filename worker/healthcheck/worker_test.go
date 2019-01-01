package healthcheck_test

import (
	"context"
	"fmt"

	"github.com/concourse/concourse/worker/healthcheck"

	fakes "github.com/concourse/concourse/worker/healthcheck/healthcheckfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Worker", func() {
	var (
		containerProvider *fakes.FakeContainerProvider
		volumeProvider    *fakes.FakeVolumeProvider
		worker            *healthcheck.Worker
		err               error
	)

	BeforeEach(func() {
		containerProvider = &fakes.FakeContainerProvider{}
		volumeProvider = &fakes.FakeVolumeProvider{}

		worker = &healthcheck.Worker{
			ContainerProvider: containerProvider,
			VolumeProvider:    volumeProvider,
		}
	})

	Context("Check", func() {
		JustBeforeEach(func() {
			err = worker.Check(context.TODO())
		})

		Context("having volume creation failing", func() {
			BeforeEach(func() {
				volumeProvider.CreateReturns(nil, fmt.Errorf("an error"))
			})

			It("fails", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("having volume creation working", func() {
			BeforeEach(func() {
				volumeProvider.CreateReturns(&healthcheck.Volume{
					Handle: "handle",
					Path:   "/rootfs",
				}, nil)
			})

			It("tries to create container", func() {
				Expect(containerProvider.CreateCallCount()).To(Equal(1))
			})

			Context("having container creation failing", func() {
				BeforeEach(func() {
					containerProvider.CreateReturns(fmt.Errorf("an error"))
				})

				It("fails", func() {
					Expect(err).To(HaveOccurred())
				})

				It("tries to delete volume", func() {
					Expect(volumeProvider.DestroyCallCount()).To(Equal(1))
				})
			})

			Context("having container creation working", func() {
				It("tries to delete container", func() {
					Expect(containerProvider.DestroyCallCount()).To(Equal(1))
				})

				Context("having container destruction failing", func() {
					BeforeEach(func() {
						containerProvider.DestroyReturns(fmt.Errorf("an error"))
					})

					It("fails", func() {
						Expect(err).To(HaveOccurred())
					})
				})

				Context("having container destruction working", func() {
					It("tries to delete volume", func() {
						Expect(volumeProvider.DestroyCallCount()).To(Equal(1))
					})

					Context("having volume destruction failing", func() {
						BeforeEach(func() {
							volumeProvider.DestroyReturns(fmt.Errorf("an error"))
						})

						It("fails", func() {
							Expect(err).To(HaveOccurred())
						})
					})

					Context("having volume destruction succeeding", func() {
						It("succeeds", func() {
							Expect(err).NotTo(HaveOccurred())
						})

					})
				})
			})
		})

	})
})
